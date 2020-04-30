#!/bin/bash

cmd_ip=192.168.2.2
cmd_port=8080


function checkPing() {
  ssh -i /home/work/.ssh/id_rsa -o "StrictHostKeyChecking no" -n root@${1} "ping -c 1 -w 1 ${cmd_ip} &>/dev/null" > /dev/null 2>&1
  if [ $? -eq 0 ] ; then
    echo -n "remote ping ok"
    exit 0
  else
    echo -n "remote ping error"
    exit 100
  fi
}

function checkPort(){
  ssh -i /home/work/.ssh/id_rsa -o "StrictHostKeyChecking no" -n root@${1} "nc -z -w 2 ${cmd_ip} ${cmd_port} &>/dev/null" > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo "remote telnet port ok"
    exit 0
  else
    echo "remote telnet port error"
    exit 200
  fi
}

function checkSSH(){
  nc -z -w 2 ${1} 22 >/dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo -n "ssh port ok"
    exit 0
  else
    echo -n "ssh port error"
    exit 200
  fi
}


function localPing() {
  ping -c 1 -w 1 ${1} &>/dev/null
  if [ $? -eq 0 ]; then
    echo "localPingOk"
    exit 0
  else
    echo "local ping error"
    exit 300
  fi
}

function main() {
  method=${1}
  host=${2}

  case "${method}" in
    "local")
      localPing ${host}
      ;;
    "check")
      checkPing ${host}
      checkPort ${host}
      ;;
    "ssh")
      checkSSH ${host}
      ;;
    "install")
      DLURL="http://192.168.2.3:8080"
      DLGOPATH="/install_dir/service"
      ssh -i /home/work/.ssh/id_rsa -o "StrictHostKeyChecking no" -n root@${host} "wget -q ${DLURL}/${DLGOPATH}/consul_install.sh -O /tmp/consul_install.sh && bash -x /tmp/consul_install.sh" >> ./install.log 2>&1
      if [ $? -eq 0 ]; then
        echo -n "consul install ok"
        exit 0
      else
        echo -n "consul install error"
        exit 400
      fi
     ;;
  esac
}

main "$@"