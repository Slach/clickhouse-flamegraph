# -*- mode: ruby -*-
# vi: set ft=ruby :
Vagrant.configure(2) do |config|
  config.vm.box = "ubuntu/focal64"
  config.vm.box_check_update = false

  if Vagrant.has_plugin?("vagrant-vbguest")
    config.vbguest.auto_update = false
  end

  if Vagrant.has_plugin?("vagrant-timezone")
    config.timezone.value = "UTC"
  end

  config.vm.define :clickhouse_flamegraph do |clickhouse_flamegraph|
    clickhouse_flamegraph.vm.network "private_network", ip: "172.16.2.77", nic_type: "virtio"
    clickhouse_flamegraph.vm.host_name = "local-flamegraph-clickhouse-pro"
  end

  config.vm.provider "virtualbox" do |vb|
    vb.gui = false
    vb.memory = "2048"
    # see https://bugs.launchpad.net/cloud-images/+bug/1874453
    vb.customize [ "modifyvm", :id, "--uartmode1", "file", File::NULL ]
   end

  config.vm.provision "shell", inline: <<-SHELL
    set -xeuo pipefail
    export DEBIAN_FRONTEND=noninteractive
    sysctl net.ipv6.conf.all.forwarding=1
    apt-get update
    apt-get install -y apt-transport-https ca-certificates software-properties-common curl
    # clickhouse
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys E0C56BD4
    add-apt-repository "deb http://repo.yandex.ru/clickhouse/deb/stable/ main/"
    # docker
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 8D81803C0EBFCD88
    add-apt-repository "deb https://download.docker.com/linux/ubuntu focal edge"
    # golang
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 52B59B1571A79DBC054901C0F6BC817356A3D45E
    add-apt-repository ppa:longsleep/golang-backports
    apt-get update

    apt-get install -y golang-1.15
    export GOPATH=/home/ubuntu/go/
    grep -q -F 'export GOPATH=$GOPATH' /home/ubuntu/.bashrc  || echo "export GOPATH=$GOPATH" >> /home/ubuntu/.bashrc
    grep -q -F 'export GOPATH=$GOPATH' /home/vagrant/.bashrc || echo "export GOPATH=$GOPATH" >> /home/vagrant/.bashrc
    grep -q -F 'export GOPATH=$GOPATH' /root/.bashrc         || echo "export GOPATH=$GOPATH" >> /root/.bashrc
    export GOROOT=/usr/lib/go-1.15/
    grep -q -F 'export GOROOT=$GOROOT' /home/ubuntu/.bashrc  || echo "export GOROOT=$GOROOT" >> /home/ubuntu/.bashrc
    grep -q -F 'export GOROOT=$GOROOT' /home/vagrant/.bashrc || echo "export GOROOT=$GOROOT" >> /home/vagrant/.bashrc
    grep -q -F 'export GOROOT=$GOROOT' /root/.bashrc         || echo "export GOROOT=$GOROOT" >> /root/.bashrc

    apt-get install --no-install-recommends -y docker-ce
    apt-get install --no-install-recommends -y clickhouse-client
    apt-get install --no-install-recommends -y python3-pip
    apt-get install --no-install-recommends -y htop ethtool mc curl wget rpm

    python3 -m pip install -U pip
    whereis pip3
    rm -rf /usr/bin/pip3
    pip3 install -U setuptools
    pip3 install -U bump2version
    pip3 install -U docker-compose
    mkdir -p /home/ubuntu/go/src/github.com/Slach/
    ln -nsfv /usr/lib/go-1.15/bin/go /usr/bin/go
    ln -nsfv /vagrant /home/ubuntu/go/src/github.com/Slach/clickhouse-flamegraph

    rm -rf /opt/flamegraph && mkdir -p /opt/flamegraph/
    git clone https://github.com/brendangregg/FlameGraph.git /opt/flamegraph/
    ln -vsf /opt/flamegraph/flamegraph.pl /usr/bin/flamegraph.pl
    ln -vsf /opt/flamegraph/flamegraph.pl /vagrant/flamegraph.pl

    goreleaser_urls=$(curl -sL https://github.com/goreleaser/goreleaser/releases/latest | grep href | grep -E "amd64\\.deb|\\.txt" | cut -d '"' -f 2)
    echo "$goreleaser_urls" > /tmp/goreleaser_urls.txt
    sed -i -e "s/^\\/goreleaser/https:\\/\\/github.com\\/goreleaser/" /tmp/goreleaser_urls.txt
    wget -nv -c -i /tmp/goreleaser_urls.txt
    grep amd64.deb goreleaser_checksums.txt | sha256sum
    dpkg -i $(cat goreleaser*checksums.txt | grep amd64.deb | cut -d " " -f 2-)

    nfpm_urls=$(curl -sL https://github.com/goreleaser/nfpm/releases/latest | grep href | grep -E "amd64\\.deb|\\.txt" | cut -d '"' -f 2)
    echo "$nfpm_urls" > /tmp/nfpm_urls.txt
    sed -i -e "s/^\\/goreleaser/https:\\/\\/github.com\\/goreleaser/" /tmp/nfpm_urls.txt
    wget -nv -c -i /tmp/nfpm_urls.txt
    grep amd64.deb nfpm*checksums.txt | sha256sum
    dpkg -i $(cat nfpm*checksums.txt | grep amd64.deb | cut -d " " -f 2-)

    mkdir -p -m 0700 /root/.ssh/
    cp -fv /vagrant/id_rsa /root/.ssh/id_rsa
    chmod 0600 /root/.ssh/id_rsa
    touch /root/.ssh/known_hosts
    ssh-keygen -R github.com
    ssh-keygen -R bitbucket.org
    ssh-keyscan -H github.com >> /root/.ssh/known_hosts
    ssh-keyscan -H bitbucket.org >> /root/.ssh/known_hosts

    git config --global url."git@github.com:".insteadOf "https://github.com/"
    git config --global url."git@bitbucket.org:".insteadOf "https://bitbucket.org/"

    cd /vagrant/
    docker-compose down
    docker system prune -f
    docker volume prune -f
    docker-compose up -d
  SHELL
end
