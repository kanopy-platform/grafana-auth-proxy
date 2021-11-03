#!/bin/bash

cat <<EOF > "${GIT_ROOT}/go.mod"
module ${GO_MODULE}

go ${GO_VERSION}

require (
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
)
EOF
