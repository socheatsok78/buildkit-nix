package main

import (
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/sirupsen/logrus"
	"github.com/socheatsok78/buildkit-nix/nixfile"
)

func main() {
	err := grpcclient.RunFromEnvironment(appcontext.Context(), nixfile.Build)
	if err != nil {
		logrus.Fatal(err)
	}
}
