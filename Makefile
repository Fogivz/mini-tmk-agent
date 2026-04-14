build:
	go build -o go-trans

install: build
	sudo mv go-trans /usr/local/bin/
	sudo chmod +x /usr/local/bin/go-trans
