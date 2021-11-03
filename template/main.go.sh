#!/bin/bash

cat <<EOF > "${GIT_ROOT}/main.go"
package main

import (
	"${GO_MODULE}/internal/cli"
	log "github.com/sirupsen/logrus"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		log.Fatalln(err)
	}
}
EOF
