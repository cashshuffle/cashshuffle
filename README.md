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

## License

cashshuffle is released under the MIT license.

[License-URL]: http://opensource.org/licenses/MIT
[License-Image]: https://img.shields.io/npm/l/express.svg
[ReportCard-URL]: http://goreportcard.com/report/cashshuffle/cashshuffle
[ReportCard-Image]: https://goreportcard.com/badge/github.com/cashshuffle/cashshuffle
[Build-Status-URL]: http://travis-ci.org/cashshuffle/cashshuffle
[Build-Status-Image]: https://travis-ci.org/cashshuffle/cashshuffle.svg?branch=master
