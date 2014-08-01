#!/bin/bash
# Bowery Delancey agent install script
# curl -s bowery.sh | bash

set -e

# for colorful terminal output
function colecho {
    local GREEN="\033[32m"
    local RED="\033[91m"
    local CYAN="\033[36m"
    local NONE="\033[0m"

    case $1 in
        -g|--green)
            printf "$GREEN${*:2}" ;;
        -r|--red)
            printf "$RED${*:2}" ;;
        -c|--cyan)
            printf "$CYAN${*:2}" ;;
        *)
            printf "${*}" ;;
    esac

    printf "$NONE\n"
}

function on_error {
    colecho -r "Bowery agent installation failed."
}
trap on_error ERR

function script_error {
    if [[ -n $ERR_STR ]]; then echo $ERR_STR; fi
}
trap script_error EXIT

OS_ERR="Sorry, we do not support this server's operating system."
LNX_DSTR_DTCTN_ERR="Sorry, we cannot detect this server's Linux distribution."

function os_error {
    ERR_STR="$OS_ERR"

    if [[ -n $OS ]]; then ERR_STR="$ERR_STR $OS"; fi
    if [[ -n $NAME ]]; then ERR_STR="$ERR_STR $NAME"; fi

    on_error
    exit 1
}

# linux_install is called after the binaries are installed. Initiates the service
function linux_install {
    if [[ -e /etc/os-release ]] # pretty much on everything except VMs
    then
        # exposes distro as $NAME
        . /etc/os-release
    elif [[ -e /proc/version ]] # definitely everywhere
    then
        NAME="$(cat /proc/version)"
    else
        ERR_STR=$LNX_DSTR_ERR
        exit 1
    fi

    # for better error messaging
    case $NAME in
        *Fedora*)
            NAME=Fedora ;;
        *CentOS*)
            NAME=CentOS ;;
        *Red\ Hat*)
            NAME=RedHat ;;
        *SUSE*)
            NAME=SUSE;;
        *Ubuntu*)
            NAME=Ubuntu ;;
        *Debian*)
            NAME=Debian ;;
        *)
            os_error ;;
    esac

    case $NAME in
        "Fedora")
            # move the bash file in
            # start the srvice
            os_error ;;
        "CentOS"|"RedHat")
            os_error ;;
        "SUSE")
            os_error ;;
        "Ubuntu"|"Debian")
            curl -so $dir/bowery-agent.conf http://${bucket}.${s3url}/agent.conf
            sudo mv $dir/bowery-agent.conf /etc/init/
            sudo service bowery-agent start ;;
    esac
}

function darwin_install {
    # download plist file
    # move it where it needs to go
    :
}

function solaris_install {
    :
}

colecho -g "Thanks for using Bowery!"

dir=/tmp/bowery
mkdir -p $dir

# figure out operating system...
case $OSTYPE in
    *linux*)
        OS=linux ;;
    *darwin*)
        OS=darwin
        os_error ;;
    *solaris*)
        OS=solaris
        os_error ;;
    *)
        os_error ;;
esac

# figure out architecture...
case "$(uname -m)" in
    "x86_64")
        ARCH=amd64 ;;
    *arm*)
        ARCH=arm ;;
    *)
        ARCH=386 ;;
esac

bucket=bowery.sh
s3url=s3.amazonaws.com
VERSION=$(curl -s http://${bucket}.${s3url}/VERSION)

printf "Downloading agent... "
curl -so $dir/bowery-agent.tar.gz http://${bucket}.${s3url}/${VERSION}_${OS}_${ARCH}.tar.gz
printf "Installing... "
tar -xzf $dir/bowery-agent.tar.gz
mv agent $dir/bowery-agent
sudo mv $dir/bowery-agent /usr/local/bin/
colecho -c "Done!"

printf "Setting up daemon... "
case $OS in
    "linux")
        linux_install ;;
    "darwin")
        darwin_install ;;
    "solaris")
        solaris_install ;;
esac

colecho -c "Done!"
exit 0
