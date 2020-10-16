package assets

 // PerunTokenBinRuntime is the runtime part of the compiled bytecode used for deploying new contracts.
var PerunTokenBinRuntime = `608060405234801561001057600080fd5b50600436106100a95760003560e01c80633950935111610071578063395093511461012957806370a082311461013c57806395d89b411461014f578063a457c2d714610157578063a9059cbb1461016a578063dd62ed3e1461017d576100a9565b806306fdde03146100ae578063095ea7b3146100cc57806318160ddd146100ec57806323b872dd14610101578063313ce56714610114575b600080fd5b6100b6610190565b6040516100c39190610753565b60405180910390f35b6100df6100da36600461071f565b610226565b6040516100c39190610748565b6100f4610243565b6040516100c391906108eb565b6100df61010f3660046106e4565b610249565b61011c6102d0565b6040516100c391906108f4565b6100df61013736600461071f565b6102d9565b6100f461014a366004610698565b610327565b6100b6610346565b6100df61016536600461071f565b6103a7565b6100df61017836600461071f565b61040f565b6100f461018b3660046106b2565b610423565b60038054604080516020601f600260001961010060018816150201909516949094049384018190048102820181019092528281526060939092909183018282801561021c5780601f106101f15761010080835404028352916020019161021c565b820191906000526020600020905b8154815290600101906020018083116101ff57829003601f168201915b5050505050905090565b600061023a610233610483565b8484610487565b50600192915050565b60025490565b600061025684848461053b565b6102c684610262610483565b6102c185604051806060016040528060288152602001610929602891396001600160a01b038a166000908152600160205260408120906102a0610483565b6001600160a01b031681526020810191909152604001600020549190610650565b610487565b5060019392505050565b60055460ff1690565b600061023a6102e6610483565b846102c185600160006102f7610483565b6001600160a01b03908116825260208083019390935260409182016000908120918c16815292529020549061044e565b6001600160a01b0381166000908152602081905260409020545b919050565b60048054604080516020601f600260001961010060018816150201909516949094049384018190048102820181019092528281526060939092909183018282801561021c5780601f106101f15761010080835404028352916020019161021c565b600061023a6103b4610483565b846102c18560405180606001604052806025815260200161095160259139600160006103de610483565b6001600160a01b03908116825260208083019390935260409182016000908120918d16815292529020549190610650565b600061023a61041c610483565b848461053b565b6001600160a01b03918216600090815260016020908152604080832093909416825291909152205490565b60008282018381101561047c5760405162461bcd60e51b81526004016104739061082b565b60405180910390fd5b9392505050565b3390565b6001600160a01b0383166104ad5760405162461bcd60e51b8152600401610473906108a7565b6001600160a01b0382166104d35760405162461bcd60e51b8152600401610473906107e9565b6001600160a01b0380841660008181526001602090815260408083209487168084529490915290819020849055517f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b9259061052e9085906108eb565b60405180910390a3505050565b6001600160a01b0383166105615760405162461bcd60e51b815260040161047390610862565b6001600160a01b0382166105875760405162461bcd60e51b8152600401610473906107a6565b61059283838361067c565b6105cf81604051806060016040528060268152602001610903602691396001600160a01b0386166000908152602081905260409020549190610650565b6001600160a01b0380851660009081526020819052604080822093909355908416815220546105fe908261044e565b6001600160a01b0380841660008181526020819052604090819020939093559151908516907fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9061052e9085906108eb565b600081848411156106745760405162461bcd60e51b81526004016104739190610753565b505050900390565b505050565b80356001600160a01b038116811461034157600080fd5b6000602082840312156106a9578081fd5b61047c82610681565b600080604083850312156106c4578081fd5b6106cd83610681565b91506106db60208401610681565b90509250929050565b6000806000606084860312156106f8578081fd5b61070184610681565b925061070f60208501610681565b9150604084013590509250925092565b60008060408385031215610731578182fd5b61073a83610681565b946020939093013593505050565b901515815260200190565b6000602080835283518082850152825b8181101561077f57858101830151858201604001528201610763565b818111156107905783604083870101525b50601f01601f1916929092016040019392505050565b60208082526023908201527f45524332303a207472616e7366657220746f20746865207a65726f206164647260408201526265737360e81b606082015260800190565b60208082526022908201527f45524332303a20617070726f766520746f20746865207a65726f206164647265604082015261737360f01b606082015260800190565b6020808252601b908201527f536166654d6174683a206164646974696f6e206f766572666c6f770000000000604082015260600190565b60208082526025908201527f45524332303a207472616e736665722066726f6d20746865207a65726f206164604082015264647265737360d81b606082015260800190565b60208082526024908201527f45524332303a20617070726f76652066726f6d20746865207a65726f206164646040820152637265737360e01b606082015260800190565b90815260200190565b60ff9190911681526020019056fe45524332303a207472616e7366657220616d6f756e7420657863656564732062616c616e636545524332303a207472616e7366657220616d6f756e74206578636565647320616c6c6f77616e636545524332303a2064656372656173656420616c6c6f77616e63652062656c6f77207a65726fa264697066735822122094576c6ba5c17de6a9dfce9a89bdf707dfe294dc4ff2a5bcc403bcb7e65408a764736f6c63430007030033`