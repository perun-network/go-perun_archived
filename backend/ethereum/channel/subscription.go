package channel

import (
	"context"
	"math/big"
	"runtime"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/pkg/errors"
	"perun.network/go-perun/backend/ethereum/bindings/adjudicator"
	"perun.network/go-perun/backend/ethereum/wallet"
	"perun.network/go-perun/channel"
)

// Subscribe creates a new adjudicator event subscription.
func (a *Adjudicator) Subscribe(ctx context.Context, params *channel.Params) (channel.AdjudicatorSubscription, error) {
	watchOpts, err := a.NewWatchOpts(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "creating watchopts")
	}

	stored := make(chan *adjudicator.AdjudicatorStored)
	storedSub, err := a.contract.WatchStored(watchOpts, stored, []channel.ID{params.ID()})
	if err != nil {
		return nil, errors.WithMessage(err, "creating subscription on event Stored")
	}

	adjudicatorSub := makeAdjudicatorSub(storedSub)

	go func() {
		exit := func(err error) {
			close(adjudicatorSub.next)
			adjudicatorSub.err <- err
			runtime.Goexit()
		}

		e, err := a.getMostRecentEvent(ctx, params)
		if err != nil {
			exit(err)
		} else if e != nil {
			adjudicatorSub.next <- e
		}

		defer storedSub.Unsubscribe()
		for {
			select {
			case err := <-storedSub.Err():
				exit(err)

			case e := <-stored:
				_e, err := a.convertEvent(ctx, e)
				if err != nil {
					exit(err)
				}

				adjudicatorSub.next <- _e
			}
		}
	}()

	return adjudicatorSub, nil
}

func (a *Adjudicator) getMostRecentEvent(ctx context.Context, params *channel.Params) (channel.Event, error) {
	// Filter old Events
	filterOpts, err := a.NewFilterOpts(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "creating filter opts")
	}
	iter, err := a.contract.FilterStored(filterOpts, []channel.ID{params.ID()})
	if err != nil {
		return nil, errors.WithMessage(err, "filtering stored events")
	}
	defer iter.Close()

	// Fast-forward to most recent event
	for iter.Next() {
	}

	if iter.Event == nil {
		return nil, nil
	}

	e, err := a.convertEvent(ctx, iter.Event)
	if err != nil {
		return nil, errors.WithMessage(err, "converting event")
	}
	return e, nil
}

// AdjudicatorSub implements the channel.AdjudicatorSubscription interface.
type AdjudicatorSub struct {
	sub  event.Subscription // Stored event subscription
	next chan channel.Event // Registered event sink
	err  chan error         // error from subscription
	// done chan struct{}
}

func makeAdjudicatorSub(ethSub event.Subscription) AdjudicatorSub {
	return AdjudicatorSub{ethSub, make(chan channel.Event), make(chan error, 1)}
}

// Close closes the subscription.
func (sub AdjudicatorSub) Close() error {
	sub.sub.Unsubscribe()
	return nil
}

// Err blocks until an error occurrs and then returns it.
func (sub AdjudicatorSub) Err() error {
	return <-sub.err
}

// Next blocks until the next event and returns it.
func (sub AdjudicatorSub) Next() channel.Event {
	return <-sub.next
}

func (a *Adjudicator) convertEvent(ctx context.Context, e *adjudicator.AdjudicatorStored) (channel.Event, error) {
	base := channel.MakeEventBase(e.ChannelID, NewBlockTimeout(a.ContractInterface, e.Timeout))
	switch e.Phase {
	case phaseDispute:
		return &channel.RegisteredEvent{EventBase: base, Version: e.Version}, nil

	case phaseForceExec:
		args, err := a.fetchProgressCallData(ctx, e.Raw.TxHash)
		if err != nil {
			return nil, errors.WithMessage(err, "fetching call data")
		}
		app, err := channel.Resolve(wallet.AsWalletAddr(args.Params.App))
		if err != nil {
			return nil, errors.WithMessage(err, "resolving app")
		}
		newState := FromEthState(app, &args.State)
		return &channel.ProgressedEvent{
			EventBase: base,
			State:     &newState,
			Idx:       channel.Index(args.ActorIdx.Uint64()),
		}, nil

	case phaseConcluded:
		return &channel.ConcludedEvent{EventBase: base}, nil

	default:
		panic("unknown phase")
	}
}

const (
	phaseDispute = iota
	phaseForceExec
	phaseConcluded
)

type progressCallData struct {
	Params   adjudicator.ChannelParams
	StateOld adjudicator.ChannelState
	State    adjudicator.ChannelState
	ActorIdx *big.Int
	Sig      []byte
}

func (a *Adjudicator) fetchProgressCallData(ctx context.Context, txHash common.Hash) (*progressCallData, error) {
	tx, _, err := a.ContractBackend.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, errors.WithMessage(err, "getting transaction")
	}

	contract, err := abi.JSON(strings.NewReader(adjudicator.AdjudicatorABI))
	if err != nil {
		return nil, errors.WithMessage(err, "parsing adjudicator ABI")
	}

	method := contract.Methods["progress"]
	argsData := tx.Data()[len(method.ID):]

	argsI, err := method.Inputs.UnpackValues(argsData)
	if err != nil {
		return nil, errors.WithMessage(err, "unpacking")
	}

	var args progressCallData
	err = method.Inputs.Copy(&args, argsI)
	if err != nil {
		return nil, errors.WithMessage(err, "copying into struct")
	}

	return &args, nil
}
