cd /var/vcap/data/packages/
mkdir et
cd et
wget https://go.googlecode.com/files/go1.2.1.linux-amd64.tar.gz
tar xfz go1.2.1.linux-amd64.tar.gz
mkdir gopath
mkdir gopath/{bin,src,pkg}
export GOROOT=$PWD/go GOPATH=$PWD/gopath
export PATH=$PATH:$GOROOT/bin:$GOPATH/bin:$PWD
apt-get install git-core
apt-get install tmux
go get github.com/pivotal-cf-experimental/executor-tester
wget http://stedolan.github.io/jq/download/linux64/jq
chmod +x ./jq
export DATADOG_API_KEY=XXX
export DATADOG_APP_KEY=XXX
echo "Now go get your DATADOG_API_KEY and DATADOG_APP_KEY from https://app.datadoghq.com/account/settings#api"
