#!/bin/bash

set -e

# Download solc.
wget -nc "https://github.com/ethereum/solidity/releases/download/v0.7.0/solc-static-linux"
chmod +x solc-static-linux
git submodule update --init --recursive # this is idempotent.
echo -e "Ensure that the newest version of abigen is installed"

# Generates optimized golang bindings and runtime binaries for sol contracts.
# $1  solidity file path, relative to ../contracts/contracts/.
# $1  golang package name.
# $2… list of contract names.
function generate() {
    file=$1; pkg=$2
    shift; shift   # skip the first two args.
    for contract in "$@"; do
        abigen --pkg $pkg --sol ../contracts/contracts/$file.sol --out $pkg/$file.go --solc ./solc-static-linux
        ./solc-static-linux --bin-runtime --optimize --allow-paths *, ../contracts/contracts/$file.sol --overwrite -o $pkg/
        echo -e "package $pkg\n\n // ${contract}BinRuntime is the runtime part of the compiled bytecode used for deploying new contracts.\nvar ${contract}BinRuntime = \"$(<${pkg}/${contract}.bin-runtime)\"" > "$pkg/${contract}BinRuntime.go"
    done
}

# Adjudicator
generate "Adjudicator" "adjudicator" "Adjudicator"

# PerunToken, AssetHolderETH and AssetHolderERC20
cat ../contracts/contracts/PerunToken.sol \
    <(tail -n +17 ../contracts/contracts/AssetHolderETH.sol) \
    <(tail -n +17 ../contracts/contracts/AssetHolderERC20.sol) \
    > ../contracts/contracts/Contracts.sol
generate "Contracts" "assets" "PerunToken" "AssetHolderETH" "AssetHolderERC20"

abigen --version --solc ./solc-static-linux
echo -e "Generated bindings"
