build:
	go build -o mini-tmk-agent

install: build
	sudo mv mini-tmk-agent /usr/local/bin/
	sudo chmod +x /usr/local/bin/mini-tmk-agent
