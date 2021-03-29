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
