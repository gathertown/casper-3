# casper-3

## Description

The application subscribes to the kubernetes API feed and monitors for predefined labels on kubernetes nodes.
When a node featuring the predefined label is found, a DNS `A` record alongside a `TXT` record will be
created based on the DNS provider. Conversely the application will delete DNS entries that don't match existing nodes.

## Supported Providers

* Digital Ocean
* CloudFlare

##  Development

TODO

## Deployment

TODO


## Trivia

The S.C. Magi System (マギ) are a trio of supercomputers designed by Dr. Naoko Akagi during her research into bio-computers while at Gehirn.
[Casper-3](https://evangelion.fandom.com/wiki/Magi) is one of the three _Magi_.

