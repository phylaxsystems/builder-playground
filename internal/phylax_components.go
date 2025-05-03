package internal

type AssertionDA struct {
	devMode bool
	pk      string
}

func (a *AssertionDA) Run(service *service, ctx *ExContext) {
	var name string
	if a.devMode {
		name = "ghcr.io/phylaxsystems/assertion-da/assertion-da-dev"
	} else {
		name = "ghcr.io/phylaxsystems/assertion-da/assertion-da"
	}
	service.
		WithImage(name).
		WithTag("main").
		WithArgs("listen-addr", "0.0.0.0"+`{{Port "http" 5000}}`, "--private-key", a.pk)
}

func (a *AssertionDA) Name() string {
	if a.devMode {
		return "assertion-da-dev"
	}
	return "assertion-da"
}

type OpTalos struct {
	AssertionDA string
}

func (o *OpTalos) Run(service *service, ctx *ExContext) {
	service.WithImage("ghcr.io/phylaxsystems/op-talos/op-rbuilder").
		WithTag("main").
		WithArgs(
			"node",
			"--authrpc.port", `{{Port "authrpc" 8551}}`,
			"--authrpc.addr", "0.0.0.0",
			"--authrpc.jwtsecret", "{{.Dir}}/jwtsecret",
			"--http",
			"--http.addr", "0.0.0.0",
			"--http.port", `{{Port "http" 8545}}`,
			"--chain", "{{.Dir}}/l2-genesis.json",
			"--datadir", "{{.Dir}}/data_op_reth",
			"--disable-discovery",
			"--color", "never",
			"--metrics", `0.0.0.0:{{Port "metrics" 9090}}`,
			"--port", `{{Port "rpc" 30303}}`,
			"--ae.rpc_da_url", Connect(o.AssertionDA, "http"),
		)
}

func (o *OpTalos) Name() string {
	return "op-talos"
}
