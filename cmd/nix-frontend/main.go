package main

import (
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
	builder "github.com/socheatsok78/buildkit-nix"
)

func main() {
	err := grpcclient.RunFromEnvironment(appcontext.Context(), builder.Build)
	if err != nil {
		logrus.Fatal(err)
	}
}
