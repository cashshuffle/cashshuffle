[![License][License-Image]][License-URL] [![ReportCard][ReportCard-Image]][ReportCard-URL] [![Build][Build-Status-Image]][Build-Status-URL]
# cashshuffle

A CashShuffle server implented in Go.

For more information on CashShuffle visit [cashshuffle.com](https://cashshuffle.com).

Technical specification and documentation are at [github.com/cashshuffle/spec](https://github.com/cashshuffle/spec).

## Install

```
go get -v github.com/cashshuffle/cashshuffle
cd $GOPATH/src/github.com/cashshuffle/cashshuffle
export GO111MODULE=on
make
make install
```

## Usage

To start the server, just set the pool size and add your SSL cert and key.

```
cashshuffle -s 5 -c <cert> -k <key>
```

To start the server using LetsEncrypt to manage the cert.

```
# LetsEncrypt requires port 80 for negotiation.
# Therefore sudo is required.
sudo cashshuffle -s 5 -a <hostname>
```

You can use `--help` to view all the options.

```
Usage:
  cashshuffle [flags]

Flags:
  -a, --auto-cert string         register hostname with LetsEncrypt
  -b, --bind-ip string           IP address to bind to
  -c, --cert string              path to server.crt for TLS
  -d, --debug                    debug mode
  -h, --help                     help for cashshuffle
  -k, --key string               path to server.key for TLS
  -s, --pool-size int            pool size (default 5)
  -p, --port int                 server port (default 1337)
  -z, --stats-port int           stats server port (default 8080)
  -t, --tor                      enable secondary listener for tor connections
      --tor-bind-ip string       IP address to bind to for tor (default "127.0.0.1")
      --tor-port int             tor server port (default 1339)
      --tor-stats-port int       tor stats server port (default 8081)
      --tor-websocket-port int   tor websocket port (default 1340)
  -v, --version                  display version
  -w, --websocket-port int       websocket port (default 1338)
```

## Tor

To run a server on the public internet with SSL and also support Tor just use the `--tor` flag.

```
cashshuffle -s 5 -c <cert> -k <key> --tor
```

Now edit your `torrc` and add the following. Then restart Tor for the configuration to take effect.

```
HiddenServiceDir /var/lib/tor/cashshuffle
HiddenServicePort 1339 127.0.0.1:1339
HiddenServicePort 1340 127.0.0.1:1340
HiddenServicePort 8081 127.0.0.1:8081
```

For more docs on setting up onion services you can check out https://www.torproject.org/docs/tor-onion-service.html.en.

## License

cashshuffle is released under the MIT license.

[License-URL]: http://opensource.org/licenses/MIT
[License-Image]: https://img.shields.io/npm/l/express.svg
[ReportCard-URL]: http://goreportcard.com/report/cashshuffle/cashshuffle
[ReportCard-Image]: https://goreportcard.com/badge/github.com/cashshuffle/cashshuffle
[Build-Status-URL]: http://travis-ci.com/cashshuffle/cashshuffle
[Build-Status-Image]: https://travis-ci.com/cashshuffle/cashshuffle.svg?branch=master
