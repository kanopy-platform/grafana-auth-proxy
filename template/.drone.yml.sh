#!/bin/bash

cat <<EOF > "${GIT_ROOT}/.drone.yml"
---
kind: pipeline
type: kubernetes
name: default

trigger:
  branch: [main]

workspace:
  path: /go/src/github.com/\${DRONE_REPO}

volumes:
  - name: cache
    temp: {}

steps:
  - name: test
    image: ${GOLANGCI_IMAGE}
    volumes:
      - name: cache
        path: /go
    commands:
      - apk add make
      - make test

  - name: build
    image: plugins/kaniko-ecr
    pull: always
    volumes:
      - name: cache
        path: /go
    settings:
      no_push: true
    when:
      event: [pull_request]

  - name: publish
    image: plugins/kaniko-ecr
    pull: always
    volumes:
      - name: cache
        path: /go
    settings:
      create_repository: true
      repo: \${DRONE_REPO_NAME}
      tags:
        - git-\${DRONE_COMMIT_SHA:0:7}
        - latest
      access_key:
        from_secret: ecr_access_key
      secret_key:
        from_secret: ecr_secret_key
    when:
      event: [push]

  - name: publish-tag
    image: plugins/kaniko-ecr
    pull: always
    volumes:
      - name: cache
        path: /go
    settings:
      repo: \${DRONE_REPO_NAME}
      tags:
        - git-\${DRONE_COMMIT_SHA:0:7}
        - \${DRONE_TAG}
      access_key:
        from_secret: ecr_access_key
      secret_key:
        from_secret: ecr_secret_key
    when:
      event: [tag]
EOF
