package db

type IteratorChanWrapper struct {
	it    Iterator
	itemc chan Item
	done  chan struct{}
}

func (iw *IteratorChanWrapper) Error() error {
	return iw.it.Error()
}

func (iw *IteratorChanWrapper) Items() <-chan Item {
	return iw.itemc
}

func (iw *IteratorChanWrapper) Done() chan<- struct{} {
	return iw.done
}

func AsChanIterator(it Iterator) *IteratorChanWrapper {
	iw := &IteratorChanWrapper{
		it:    it,
		itemc: make(chan Item),
		done:  make(chan struct{}),
	}

	go func() {
		// close item channels when loop has finished
		defer close(iw.itemc)

		// iterate; if error occurs, Next() returns false
		for iw.it.Next() {
			iw.itemc <- Item{Key: iw.it.Key(), Value: iw.it.Value()}
		}
	}()

	go func() {
		<-iw.done
		it.Release()
	}()

	return iw
}
