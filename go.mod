module github.com/hyperledger/fabric

go 1.14

require (
	code.cloudfoundry.org/clock v1.0.0
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/IBM/idemix v0.0.0-20220112103229-701e7610d405
	github.com/Knetic/govaluate v3.0.0+incompatible
	github.com/Shopify/sarama v1.20.1
	github.com/Shopify/toxiproxy v2.1.4+incompatible // indirect
	github.com/SmartBFT-Go/consensus v0.0.0-20220105122543-57ed2ce2c73b
	github.com/VictoriaMetrics/fastcache v1.5.7
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190620071333-e64a0ec8b42a // indirect
	github.com/coreos/pkg v0.0.0-20180108230652-97fdf19511ea // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/frankban/quicktest v1.11.3 // indirect
	github.com/fsouza/go-dockerclient v1.7.0
	github.com/go-kit/kit v0.9.0
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.3-0.20201103224600-674baa8c7fc3 // indirect
	github.com/gorilla/handlers v1.4.0
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0
	github.com/hashicorp/go-version v1.2.0
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hyperledger/fabric-chaincode-go v0.0.0-20201119163726-f8ef75b17719
	github.com/hyperledger/fabric-config v0.1.0
	github.com/hyperledger/fabric-lib-go v1.0.0
	github.com/hyperledger/fabric-protos-go v0.0.0-20220315113721-7dc293e117f7
	github.com/kr/pretty v0.2.1
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/miekg/pkcs11 v1.0.3
	github.com/mitchellh/mapstructure v1.3.2
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/runc v1.0.0-rc8 // indirect
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/spf13/afero v1.3.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.1.1
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.7.1-0.20210116013205-6990a05d54c2 // includes ErrorContains
	github.com/sykesm/zap-logfmt v0.0.2
	github.com/syndtr/goleveldb v1.0.1-0.20210305035536-64b5b1c73954
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/willf/bitset v1.1.10
	go.etcd.io/etcd v0.5.0-alpha.5.0.20181228115726-23731bf9ba55
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.19.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
	golang.org/x/tools v0.0.0-20201201210846-92771a23d8e3
	google.golang.org/grpc v1.33.1
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gopkg.in/yaml.v2 v2.4.0
)

replace (
	github.com/SmartBFT-Go/consensus => github.com/pfi79/consensus v0.0.0-20221124213258-83763d9fff32
	github.com/cespare/xxhash/v2 => github.com/cespare/xxhash/v2 v2.1.2 // fix for Go 1.17 in github.com/prometheus/client_golang dependency without updating protobuf
	// develop
	//github.com/hyperledger/fabric => ./
	github.com/hyperledger/fabric-config => github.com/scientificideas/fabric-config v0.1.1-0.20220421194734-201061bc5f26
	github.com/hyperledger/fabric-protos-go => github.com/scientificideas/fabric-protos-go v0.0.0-20220722214515-fd27d52dbd18
	github.com/onsi/gomega => github.com/onsi/gomega v1.9.0
)
