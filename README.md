# Regenesis Tool
This tool allows to read the erigon state and create a genesis file for geth.
### Prerequisites:
First, clone this repo here (in this path):
```
git clone https://github.com/0xPolygonHermez/cdk-erigon.git
```

Command example:
```
clear && go run main.go --action=regenesis --chaindata="./erigon-datadir/cardona/chaindata" --output=./

clear && go run main.go --action=extractFullAccount --account=0x528e26b25a34a4A5d0dbDa1d57D318153d2ED583 --chaindata="./erigon-datadir/cardona/chaindata"
``` 

