[![License][License-Image]][License-URL] [![ReportCard][ReportCard-Image]][ReportCard-URL] [![Build][Build-Status-Image]][Build-Status-URL]
# cashshuffle

A CashShuffle server implented in Go. For more information on CashShuffle visit [https://cashshuffle.com](https://cashshuffle.com).

## Install

```
go get -v github.com/cashshuffle/cashshuffle
cd $GOPATH/src/github.com/cashshuffle/cashshuffle
make
make install
```

If you have issues building `cashshuffle`, you can vendor the dependencies by using [gvt](https://github.com/FiloSottile/gvt):

```
go get -u github.com/FiloSottile/gvt
cd $GOPATH/src/github.com/cashshuffle/cashshuffle
gvt restore
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
sudo cashshuffle -s 5 -a
```

You can use `--help` to view all the options.

```
Usage:
  cashshuffle [flags]

Flags:
  -a, --auto-cert string     register hostname with LetsEncrypt
  -c, --cert string          path to server.crt for TLS
  -d, --debug                debug mode
  -h, --help                 help for cashshuffle
  -k, --key string           path to server.key for TLS
  -s, --pool-size int        pool size (default 5)
  -p, --port int             server port (default 8080)
  -z, --stats-port int       stats server port (default disabled)
  -v, --version              display version
```

## License

cashshuffle is released under the MIT license.

[License-URL]: http://opensource.org/licenses/MIT
[License-Image]: https://img.shields.io/npm/l/express.svg
[ReportCard-URL]: http://goreportcard.com/report/cashshuffle/cashshuffle
[ReportCard-Image]: https://goreportcard.com/badge/github.com/cashshuffle/cashshuffle
[Build-Status-URL]: http://travis-ci.org/cashshuffle/cashshuffle
[Build-Status-Image]: https://travis-ci.org/cashshuffle/cashshuffle.svg?branch=master
