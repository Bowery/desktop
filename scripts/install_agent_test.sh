#!/bin/bash
# Bowery Agent Installer Test
echo $'\n===> Ubuntu'
ssh -l root -t 162.243.64.10 'curl -s bowery.sh | bash'

echo $'\n===> Debian'
ssh -l root -t 107.170.15.151 'curl -s bowery.sh | bash'

echo $'\n===> Fedora'
ssh -l root -t 107.170.149.45  'curl -s bowery.sh | bash'

echo $'\n===> CentOS'
ssh -l root -t 162.243.86.209 'curl -s bowery.sh | bash'

echo $'\n===> RedHat Enterprise'
ssh -i ~/.ssh/bowery.pem -l ec2-user -t 54.84.252.175 'curl -s bowery.sh | bash'

echo $'\n===> SmartOS (Solaris)'
ssh -l root -t 72.2.119.204 'curl -s bowery.sh | bash'

echo $'\n===> FreeBSD'
ssh -i ~/.ssh/bowery.pem -l ec2-user -t 54.86.97.228 'curl -s bowery.sh | bash'

# echo $'\n===> Localhost (Mac/Windows)'
# curl -s bowery.sh | bash
