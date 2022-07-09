module github.com/hyperledger/fabric

go 1.14

require (
	code.cloudfoundry.org/clock v1.0.0
	github.com/DataDog/zstd v1.4.0 // indirect
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Shopify/sarama v1.20.1
	github.com/Shopify/toxiproxy v2.1.4+incompatible // indirect
	github.com/SmartBFT-Go/consensus v0.0.0-00010101000000-000000000000
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e // indirect
	github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/docker v17.12.0-ce-rc1.0.20180827131323-0c5f8d2b9b23+incompatible // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/eapache/go-resiliency v1.1.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/frankban/quicktest v1.14.3 // indirect
	github.com/fsouza/go-dockerclient v1.3.0
	github.com/go-kit/kit v0.8.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.3
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.7.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/hashicorp/go-version v1.2.0
	github.com/hyperledger/fabric-amcl v0.0.0-20181230093703-5ccba6eab8d6
	github.com/hyperledger/fabric-lib-go v1.0.0
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pretty v0.3.0
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/miekg/pkcs11 v1.0.3
	github.com/mitchellh/mapstructure v1.1.2
	github.com/onsi/ginkgo v1.7.0
	github.com/onsi/gomega v1.5.0
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v0.9.3
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/procfs v0.0.0-20190521135221-be78308d8a4f // indirect
	github.com/rcrowley/go-metrics v0.0.0-20181016184325-3113b8401b8a
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/spf13/cast v1.3.0 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v0.0.0-20150908122457-1967d93db724
	github.com/stretchr/testify v1.7.0
	github.com/sykesm/zap-logfmt v0.0.2
	github.com/syndtr/goleveldb v1.0.1-0.20190318030020-c3a204f8e965
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/willf/bitset v1.1.10
	go.etcd.io/etcd v0.5.0-alpha.5.0.20181228115726-23731bf9ba55
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20190513172903-22d7a77e9e5f
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20190520201301-c432e742b0af // indirect
	golang.org/x/text v0.3.2 // indirect
	golang.org/x/tools v0.0.0-20191108193012-7d206e10da11
	google.golang.org/grpc v1.23.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/SmartBFT-Go/consensus => github.com/scientificideas/consensus v0.0.0-20220421192645-f6b79c2a721c
