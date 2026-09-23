#!/usr/bin/env bash

### Fixed argot version - upgrade here carefully and check that analyses are still supported
REQUIRED_ARGOT_VERSION="v0.4.3-alpha"
UPDATE_FLAG="$1"
DOCKER_FLAG="$2"


log() {
  echo "__   $1"
}

logok() {
  echo "OK   $1"
}

logfail() {
  echo "FAIL $1"
}

sep() {
  echo "======================================================"
}


argot_run_all() {
  CONFIG="$GO_SPACE/Tools/src/argot-config.yaml"
  sep
  log "Running taint"
  argot taint -config "$CONFIG"
  sep
  log "Running backtrace"
  argot backtrace -config "$CONFIG"
  sep
  log "Running syntactic"
  argot syntactic -config "$CONFIG"
}

### SCRIPT LOGIC  ###

update_argot() {
  ### Check the required argot version is present (non-zero and starts and the required full version starts with the
  # output of argot --version)
  ARGOT_VERSION=$(argot --version 2> /dev/null)
  if [[ -n $ARGOT_VERSION ]] && [[ $REQUIRED_ARGOT_VERSION == $ARGOT_VERSION* ]]; then
    logok "Argot $ARGOT_VERSION is present."
  else
    log "Installing argot ${REQUIRED_ARGOT_VERSION}"
    # Installing argot using local Go toolchain.
    # TODO: when no local toolchain, figure out how to integrate with Brazil
    
    if [ "$DOCKER_FLAG" = "True" ] || [ "$DOCKER_FLAG" = "true" ]; then
      GOBIN=/opt/amazon/agent-code go install "github.com/awslabs/ar-go-tools/cmd/argot@$REQUIRED_ARGOT_VERSION"
    else
      go install "github.com/awslabs/ar-go-tools/cmd/argot@$REQUIRED_ARGOT_VERSION"
    fi
    
    ARGOT_VERSION=$(argot --version)
    logok "Successfully installed argot $ARGOT_VERSION"
  fi
}


log "UPDATE_FLAG is '$UPDATE_FLAG'"
if [ "$UPDATE_FLAG" = "True" ] || [ "$UPDATE_FLAG" = "true" ]; then
  update_argot
else
  update_argot
  mkdir -p logs/argot
  argot_run_all ""
fi
