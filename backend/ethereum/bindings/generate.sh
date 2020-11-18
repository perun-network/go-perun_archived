#!/bin/bash

# Copyright 2020 - See NOTICE file for copyright holders.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

ABIGEN=abigen

# Download solc.
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # GNU Linux
    wget -nc "https://github.com/ethereum/solidity/releases/download/v0.7.4/solc-static-linux"
    chmod +x solc-static-linux
    SOLC=./solc-static-linux

elif [[ "$OSTYPE" == "darwin"* ]]; then
    # Mac OSX
    curl -L "https://github.com/ethereum/solidity/releases/download/v0.7.4/solc-macos" -o solc-macos
    chmod +x solc-macos
    SOLC=./solc-macos

else
    # Unsupported
    echo "$OSTYPE unsupported. Exiting."
    exit 1
fi

echo -e "Exec 'git submodule update --init --recursive' once after cloning."
echo -e "Ensure that the newest version of abigen is installed"

# Generates optimized golang bindings and runtime binaries for sol contracts.
# $1  solidity file path, relative to ../contracts/contracts/.
# $2  golang package name.
function generate() {
    FILE=$1; PKG=$2; CONTRACT=$FILE
    echo "generate package $PKG"
    abigen --pkg $PKG --sol ../contracts/contracts/$FILE.sol --out $PKG/$FILE.go --solc $SOLC
    $SOLC --bin-runtime --optimize --allow-paths *, ../contracts/contracts/$FILE.sol --overwrite -o $PKG/
    echo -e "package $PKG\n\n // ${CONTRACT}BinRuntime is the runtime part of the compiled bytecode used for deploying new contracts.\nvar ${CONTRACT}BinRuntime = \`$(<${PKG}/${CONTRACT}.bin-runtime)\`" > "$PKG/${CONTRACT}BinRuntime.go"
}

# Adjudicator
generate "Adjudicator" "adjudicator" "Adjudicator"

# PerunToken, AssetHolderETH and AssetHolderERC20
generate "PerunToken" "assets"
generate "AssetHolderETH" "assets"
generate "AssetHolderERC20" "assets"

$ABIGEN --version
echo -e "Generated bindings"
