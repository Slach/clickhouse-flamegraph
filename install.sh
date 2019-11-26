#!/usr/bin/env bash
set -xeuo pipefail
GITHUB_USER="Slach"
GITHUB_URL="https://github.com/Slach/clickhouse-flamegraph"
PACKAGE_NAME="clickhouse-flamegraph"
urls=$(curl -sL ${GITHUB_URL}/releases/latest | grep href | grep -E "\\.tar.gz|\\.rpm|\\.deb|\\.txt" | cut -d '"' -f 2)
echo "$urls" > /tmp/${PACKAGE_NAME}_urls.txt
sed -i -e "s/^\\/${GITHUB_USER}/https:\\/\\/github.com\\/${GITHUB_USER}/" /tmp/${PACKAGE_NAME}_urls.txt

if [[ "$OSTYPE" == "linux-gnu" ]]; then
    PKG_MANAGER=$( command -v yum || command -v apt-get )
    $PKG_MANAGER install -y wget
    if [[ -n "$(dpkg -l)" ]] 2>/dev/null; then
        grep -E "\\.deb|\\.txt" /tmp/${PACKAGE_NAME}_urls.txt | wget -nv -c -i -
        PKG_MANAGER_LOCAL="dpkg -i"
        PKG_CHECKSUM_FILTER="amd64.deb"
    elif [[ -n "$(rpm -qa)" ]] 2>/dev/null; then
        grep -E "\\.rpm|\\.txt" /tmp/${PACKAGE_NAME}_urls.txt | wget -nv -c -i -
        PKG_MANAGER_LOCAL="rpm -i"
        PKG_CHECKSUM_FILTER="x86_64.rpm"
    else
        grep -E "\\linux_amd64.tar.gz|\\.txt" /tmp/${PACKAGE_NAME}_urls.txt | wget -nv -c -i -
        PKG_MANAGER_LOCAL="tar -C /usr/local/bin -xfvz"
        PKG_CHECKSUM_FILTER="linux_amd64.tar.gz"
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    grep -E "\\darwin_amd64.tar.gz|\\.txt" /tmp/${PACKAGE_NAME}_urls.txt | wget -nv -c -i -
    PKG_MANAGER_LOCAL="tar -C /usr/local/bin -xfvz"
    PKG_CHECKSUM_FILTER="darwin_amd64.tar.gz"
elif [[ "$OSTYPE" == "cygwin"* || "$OS" == "Windows"* ]]; then
    grep -E "\\windows_amd64.tar.gz|\\.txt" /tmp/${PACKAGE_NAME}_urls.txt | wget -nv -c -i -
    PKG_MANAGER_LOCAL="tar -C \"C:\\Program Files\\ClikcHouse-FlameGraph\\\" -xfvz"
    PKG_CHECKSUM_FILTER="windows_amd64.tar.gz"
fi

grep ${PKG_CHECKSUM_FILTER} ${PACKAGE_NAME}_checksums.txt | sha256sum
${PKG_MANAGER_LOCAL} $( cat ${PACKAGE_NAME}_checksums.txt | grep ${PKG_CHECKSUM_FILTER} | cut -d " " -f 2- )
