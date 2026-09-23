#!/bin/bash

s3path=$1

echo "Uninstalling Amazon-ssm-agent"

# helper function to set error output
function error_exit
{
  echo "$1" 1>&2
  exit 1
}

# Check if the version we are about to uninstall is actually installed.
# If a different version is installed (e.g. the target was never installed
# due to a timeout), skip the uninstall to avoid removing the wrong version.
# This is a lock-free read-only query against the RPM database.
expectedVersion=$(rpm -qp --qf '%{VERSION}' amazon-ssm-agent.rpm 2>/dev/null)
installedVersion=$(rpm -q --qf '%{VERSION}' amazon-ssm-agent 2>/dev/null)
if [ -n "$expectedVersion" ] && [ -n "$installedVersion" ] && [ "$installedVersion" != "$expectedVersion" ]; then
  echo "Installed version $installedVersion does not match expected $expectedVersion, skipping uninstall"
  exit 0
fi

function uninstall_agent()
{
  PACKAGE_MANAGER='rpm'
  which yum 2>/dev/null
  RET_CODE=$?
  if [ ${RET_CODE} == 0 ];
  then
    PACKAGE_MANAGER='yum'
    echo "Package manager found. Using ${PACKAGE_MANAGER}  to install amazon-ssm-agent."
  fi
  
  echo "Attempting to uninstall amazon-ssm-agent using yum"
  pmOutput=$(yum -y --cacheonly remove amazon-ssm-agent 2>&1)
  pmExit=$?
  echo "Yum Output: $pmOutput"
  if [ ${pmExit} -ne 0 ]; then
    echo "Yum uninstall failed. Attemting to uninstall amazon-ssm-agent using rpm"
    pmOutput=$(rpm --erase amazon-ssm-agent 2>&1)
    pmExit=$?
  fi

  if [ "$pmExit" -ne 0 ]; then
    echo "Package manager failed with exit code '$pmExit'"
    echo "Package manager output: $pmOutput"
    exit 121
  fi
}

if [[ $(/sbin/init --version 2> /dev/null) =~ upstart ]]; then
  echo "Checking if the agent is installed"
  if [ "$(rpm -q amazon-ssm-agent)" != "package amazon-ssm-agent is not installed" ]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent"
		uninstall_agent
		sleep 1
  else
		echo "-> Agent is not installed in this instance"
  fi
elif [[ $(systemctl 2> /dev/null) =~ -\.mount ]]; then
  echo "Checking if the agent is installed"
  if [[ "$(systemctl status amazon-ssm-agent.service)" != *"Loaded: not-found"* ]]; then
		echo "-> Agent is installed in this instance"
		echo "Uninstalling the agent"
		uninstall_agent
		sleep 1
  else
		echo "-> Agent is not installed in this instance"
  fi
else
  echo "The amazon-ssm-agent is not supported on this platform. Please visit the documentation for the list of supported platforms" 1>&2
  exit 124
fi