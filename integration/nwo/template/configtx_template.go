/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package template

const DefaultConfigTx = `---
{{ with $w := . -}}
Organizations:{{ range .PeerOrgs }}
- &{{ .MSPID }}
  Name: {{ .Name }}
  ID: {{ .MSPID }}
  MSPDir: {{ $w.PeerOrgMSPDir . }}
  Policies:
    {{- if .EnableNodeOUs }}
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.peer', '{{.MSPID}}.client')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.client')
    Endorsement:
      Type: Signature
      Rule: OR('{{.MSPID}}.peer')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
    {{- else }}
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Endorsement:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
    {{- end }}
  AnchorPeers:{{ range $w.AnchorsInOrg .Name }}
  - Host: 127.0.0.1
    Port: {{ $w.PeerPort . "Listen" }}
  {{- end }}
{{- end }}
{{- range .IdemixOrgs }}
- &{{ .MSPID }}
  Name: {{ .Name }}
  ID: {{ .MSPID }}
  MSPDir: {{ $w.IdemixOrgMSPDir . }}
  MSPType: idemix
  Policies:
    {{- if .EnableNodeOUs }}
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.peer', '{{.MSPID}}.client')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.client')
    Endorsement:
      Type: Signature
      Rule: OR('{{.MSPID}}.peer')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
    {{- else }}
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Endorsement:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
    {{- end }}
{{ end }}
{{- range .OrdererOrgs }}
- &{{ .MSPID }}
  Name: {{ .Name }}
  ID: {{ .MSPID }}
  MSPDir: {{ $w.OrdererOrgMSPDir . }}
  Policies:
  {{- if .EnableNodeOUs }}
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.orderer', '{{.MSPID}}.client')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.orderer', '{{.MSPID}}.client')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
  {{- else }}
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.member')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
  {{- end }}
  OrdererEndpoints:{{ range $w.OrderersInOrg .Name }}
  - 127.0.0.1:{{ $w.OrdererPort . "Listen" }}
  {{- end }}
{{ end }}

Channel: &ChannelDefaults
  Capabilities:
    V2_0: true
  Policies: &DefaultPolicies
    Readers:
      Type: ImplicitMeta
      Rule: ANY Readers
    Writers:
      Type: ImplicitMeta
      Rule: ANY Writers
    Admins:
      Type: ImplicitMeta
      Rule: MAJORITY Admins

Profiles:{{ range .Profiles }}
  {{ .Name }}:
    {{- if .ChannelCapabilities}}
    Capabilities:{{ range .ChannelCapabilities}}
      {{ . }}: true
    {{- end}}
    Policies:
      <<: *DefaultPolicies
    {{- else }}
    <<: *ChannelDefaults
    {{- end}}
    {{- if .Orderers }}
    Orderer:
      OrdererType: {{ $w.Consensus.Type }}
      Addresses:{{ range .Orderers }}{{ with $w.Orderer . }}
      - 127.0.0.1:{{ $w.OrdererPort . "Listen" }}
      {{- end }}{{ end }}
      {{- if .Blocks}}
      BatchTimeout: {{ .Blocks.BatchTimeout }}s
      BatchSize:
        MaxMessageCount: {{ .Blocks.MaxMessageCount }}
        AbsoluteMaxBytes: {{ .Blocks.AbsoluteMaxBytes }} MB
        PreferredMaxBytes: {{ .Blocks.PreferredMaxBytes }} KB
      {{- else }}
      BatchTimeout: 1s
      BatchSize:
        MaxMessageCount: 1
        AbsoluteMaxBytes: 98 MB
        PreferredMaxBytes: 512 KB
      {{- end}}
      Capabilities:
        V2_0: true
      {{- if eq $w.Consensus.Type "kafka" }}
      Kafka:
        Brokers:{{ range $w.BrokerAddresses "HostPort" }}
        - {{ . }}
        {{- end }}
      {{- end }}
      {{- if eq $w.Consensus.Type "etcdraft" }}
      EtcdRaft:
        Options:
          TickInterval: 500ms
          SnapshotIntervalSize: 1 KB
        Consenters:{{ range .Orderers }}{{ with $w.Orderer . }}
        - Host: 127.0.0.1
          Port: {{ $w.OrdererPort . "Cluster" }}
          ClientTLSCert: {{ $w.OrdererLocalCryptoDir . "tls" }}/server.crt
          ServerTLSCert: {{ $w.OrdererLocalCryptoDir . "tls" }}/server.crt
        {{- end }}{{- end }}
      {{- end }}
      {{- if eq $w.Consensus.Type "smartbft" }}
      SmartBFT:
        Options:
          LeaderRotation: 0
          DecisionsPerLeader: 0
          RequestBatchMaxCount: 100
          RequestBatchMaxBytes: 10485760
          RequestBatchMaxInterval: 500ms
          IncomingMessageBufferSize: 1000
          RequestPoolSize: 100
          RequestForwardTimeout: 2s
          RequestComplainTimeout: 20s
          RequestAutoRemoveTimeout: 3m
          ViewChangeResendInterval: 5s
          ViewChangeTimeout: 20s
          LeaderHeartbeatTimeout: 1m
          LeaderHeartbeatCount: 10
          CollectTimeout: 1s
          SyncOnStart: false
          SpeedUpViewChange: false
          RequestMaxBytes: 512000
          RequestPoolSubmitTimeout: 5s
        Consenters:{{ range .Orderers }}{{ with $w.Orderer . }}
        - Host: 127.0.0.1
          Port: {{ $w.OrdererPort . "Cluster" }}
          ClientTLSCert: {{ $w.OrdererLocalCryptoDir . "tls" }}/server.crt
          ServerTLSCert: {{ $w.OrdererLocalCryptoDir . "tls" }}/server.crt
          MSPID: {{ $w.OrdererMSPID . }}
          Identity: {{ $w.OrdererCert . }}
          ConsenterId: {{ $w.OrdererIndex . }}
        {{- end }}{{- end }}
      {{- end }}
      Organizations:{{ range $w.OrgsForOrderers .Orderers }}
      - *{{ .MSPID }}
      {{- end }}
      Policies:
        Readers:
          Type: ImplicitMeta
          Rule: ANY Readers
        Writers:
          Type: ImplicitMeta
          Rule: ANY Writers
        Admins:
          Type: ImplicitMeta
          Rule: MAJORITY Admins
      {{- if eq $w.Consensus.Type "smartbft" }}
        BlockValidation:
          Type: ImplicitOrderer
          Rule: SMARTBFT
      {{- else }}
        BlockValidation:
          Type: ImplicitMeta
          Rule: ANY Writers
      {{- end }}
    {{- end }}
    {{- if .Consortium }}
    Consortium: {{ .Consortium }}
    Application:
      Capabilities:
      {{- if .AppCapabilities }}{{ range .AppCapabilities }}
        {{ . }}: true
        {{- end }}
      {{- else }}
        V1_3: true
      {{- end }}
      Organizations:{{ range .Organizations }}
      - *{{ ($w.Organization .).MSPID }}
      {{- end}}
      Policies:
        Readers:
          Type: ImplicitMeta
          Rule: ANY Readers
        Writers:
          Type: ImplicitMeta
          Rule: ANY Writers
        Admins:
          Type: ImplicitMeta
          Rule: MAJORITY Admins
        LifecycleEndorsement:
          Type: ImplicitMeta
          Rule: "MAJORITY Endorsement"
        Endorsement:
          Type: ImplicitMeta
          Rule: "MAJORITY Endorsement"
    {{- else }}
    Consortiums:{{ range $w.Consortiums }}
      {{ .Name }}:
        Organizations:{{ range .Organizations }}
        - *{{ ($w.Organization .).MSPID }}
        {{- end }}
    {{- end }}
    {{- end }}
{{- end }}
{{ end }}
`

const OrgUpdateConfigTxTemplate = `---
{{ with $w := . -}}
Organizations:{{ range .PeerOrgs }}
- &{{ .MSPID }}
  Name: {{ .Name }}
  ID: {{ .MSPID }}
  MSPDir: {{ $w.PeerOrgMSPDir . }}
  Policies:
    Readers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.peer', '{{.MSPID}}.client')
    Writers:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin', '{{.MSPID}}.client')
    Endorsement:
      Type: Signature
      Rule: OR('{{.MSPID}}.peer')
    Admins:
      Type: Signature
      Rule: OR('{{.MSPID}}.admin')
  AnchorPeers:{{ range $w.AnchorsInOrg .Name }}
  - Host: 127.0.0.1
    Port: {{ $w.PeerPort . "Listen" }}
  {{- end }}
{{- end }}
{{ end }}
`
