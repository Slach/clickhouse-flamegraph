# -*- mode: ruby -*-
# vi: set ft=ruby :
Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/bionic64"
  config.vm.box_check_update = false

  config.vm.define :clickhouse_flamegraph do |clickhouse_flamegraph|
        clickhouse_flamegraph.vm.network "private_network", ip: "172.16.2.77"
        clickhouse_flamegraph.vm.host_name = "local-clickhouse-clickhouse-pro"
  end

  # Provider-specific configuration so you can fine-tune various
  # backing providers for Vagrant. These expose provider-specific options.
  # Example for VirtualBox:
  #
  config.vm.provider "virtualbox" do |vb|
    # Display the VirtualBox GUI when booting the machine
    vb.gui = false

    # Customize the amount of memory on the VM:
    vb.memory = "2048"
  end
  config.vm.provision "shell", inline: <<-SHELL
    set -xeuo pipefail
    sysctl net.ipv6.conf.all.forwarding=1
    apt-get update
    apt-get install -y apt-transport-https ca-certificates software-properties-common curl
    # clickhouse
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys E0C56BD4
    add-apt-repository "deb http://repo.yandex.ru/clickhouse/deb/stable/ main/"
    # docker
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 8D81803C0EBFCD88
    add-apt-repository "deb https://download.docker.com/linux/ubuntu bionic edge"
    # golang
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 52B59B1571A79DBC054901C0F6BC817356A3D45E
    add-apt-repository ppa:longsleep/golang-backports
    apt-get update

    apt-get install -y golang-1.13
    export GOPATH=/home/ubuntu/go/
    grep -q -F 'export GOPATH=$GOPATH' /home/ubuntu/.bashrc  || echo "export GOPATH=$GOPATH" >> /home/ubuntu/.bashrc
    grep -q -F 'export GOPATH=$GOPATH' /home/vagrant/.bashrc || echo "export GOPATH=$GOPATH" >> /home/vagrant/.bashrc
    grep -q -F 'export GOPATH=$GOPATH' /root/.bashrc         || echo "export GOPATH=$GOPATH" >> /root/.bashrc
    export GOROOT=/usr/lib/go-1.13/
    grep -q -F 'export GOROOT=$GOROOT' /home/ubuntu/.bashrc  || echo "export GOROOT=$GOROOT" >> /home/ubuntu/.bashrc
    grep -q -F 'export GOROOT=$GOROOT' /home/vagrant/.bashrc || echo "export GOROOT=$GOROOT" >> /home/vagrant/.bashrc
    grep -q -F 'export GOROOT=$GOROOT' /root/.bashrc         || echo "export GOROOT=$GOROOT" >> /root/.bashrc

    apt-get install -y docker-ce
    apt-get install -y clickhouse-client
    apt-get install -y python-pip
    apt-get install -y htop ethtool mc

    python -m pip install -U pip
    pip install -U docker-compose
    mkdir -p /home/ubuntu/go/src/github.com/Slach/
    ln -nsfv /usr/lib/go-1.13/bin/go /usr/bin/go
    ln -nsfv /vagrant /home/ubuntu/go/src/github.com/Slach/clickhouse-flamegraph

    git clone https://github.com/brendangregg/FlameGraph.git /opt/flamegraph/
    ln -vsf /opt/flamegraph/flamegraph.pl /usr/bin/flamegraph.pl

    cd /vagrant/
    docker-compose down
    docker system prune -f
    docker-compose up -d
  SHELL
end
