package internal

type AssertionDA struct {
	devMode bool
	pk      string
}

func (a *AssertionDA) Run(service *service, ctx *ExContext) {
	var name string
	if a.devMode {
		name = "ghcr.io/phylax-systems/assertion-da/assertion-da-dev"
	} else {
		name = "ghcr.io/phylax-systems/assertion-da/assertion-da"
	}
	service.
		WithImage(name).
		WithTag("main").
		WithArgs("listen-addr", `{{Addr "http" 0.0.0.0:5000}}`, "--private-key", a.pk)
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
	service.WithImage("ghcr.io/phylax-systems/op-talos/op-rbuilder").
		WithTag("main").
		WithEntrypoint("op-rbuilder").
		WithArgs(
			"--http.addr", "0.0.0.0",
			"--http.port", `{{Port "http" 8545}}`,
			"--ae.rpc_da_url", Connect(o.AssertionDA, "http"),
		)
}

func (o *OpTalos) Name() string {
	return "op-talos"
}
