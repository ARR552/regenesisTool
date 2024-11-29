# Regenesis Tool
This tool allows to read the erigon state and create a genesis file for geth.
### Prerequisites:
First, clone this repo here (in this path):
```
git clone https://github.com/0xPolygonHermez/cdk-erigon.git
```

Command example:
```
clear && go run main.go --action=extractAccountsStorage --chaindata="./erigon-datadir/cardona/chaindata" --output=./
``` 

