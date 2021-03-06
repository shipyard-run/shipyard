#!/usr/bin/env bats

load _helpers

@test "server/ServiceAccount: enabled by default" {
  cd `chart_dir`
  local actual=$(helm template \
      -x templates/server-serviceaccount.yaml  \
      . | tee /dev/stderr |
      yq 'length > 0' | tee /dev/stderr)
  [ "${actual}" = "true" ]
}

@test "server/ServiceAccount: disabled with global.enabled=false" {
  cd `chart_dir`
  local actual=$(helm template \
      -x templates/server-serviceaccount.yaml  \
      --set 'global.enabled=false' \
      . | tee /dev/stderr |
      yq 'length > 0' | tee /dev/stderr)
  [ "${actual}" = "false" ]
}

@test "server/ServiceAccount: disabled with server disabled" {
  cd `chart_dir`
  local actual=$(helm template \
      -x templates/server-serviceaccount.yaml  \
      --set 'server.enabled=false' \
      . | tee /dev/stderr |
      yq 'length > 0' | tee /dev/stderr)
  [ "${actual}" = "false" ]
}

@test "server/ServiceAccount: enabled with server enabled" {
  cd `chart_dir`
  local actual=$(helm template \
      -x templates/server-serviceaccount.yaml  \
      --set 'server.enabled=true' \
      . | tee /dev/stderr |
      yq 'length > 0' | tee /dev/stderr)
  [ "${actual}" = "true" ]
}

@test "server/ServiceAccount: enabled with server enabled and global.enabled=false" {
  cd `chart_dir`
  local actual=$(helm template \
      -x templates/server-serviceaccount.yaml  \
      --set 'global.enabled=false' \
      --set 'server.enabled=true' \
      . | tee /dev/stderr |
      yq -s 'length > 0' | tee /dev/stderr)
  [ "${actual}" = "true" ]
}

#--------------------------------------------------------------------
# global.imagePullSecrets

@test "server/ServiceAccount: can set image pull secrets" {
  cd `chart_dir`
  local object=$(helm template \
      -x templates/server-serviceaccount.yaml  \
      --set 'global.imagePullSecrets[0].name=my-secret' \
      --set 'global.imagePullSecrets[1].name=my-secret2' \
      . | tee /dev/stderr)

  local actual=$(echo "$object" |
      yq -r '.imagePullSecrets[0].name' | tee /dev/stderr)
  [ "${actual}" = "my-secret" ]

  local actual=$(echo "$object" |
      yq -r '.imagePullSecrets[1].name' | tee /dev/stderr)
  [ "${actual}" = "my-secret2" ]
}
