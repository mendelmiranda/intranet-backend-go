.PHONY: build test benchmark start stop status

build:
	./servicos.sh compilar

test:
	./servicos.sh testar

benchmark:
	./servicos.sh benchmark

start:
	./servicos.sh iniciar

stop:
	./servicos.sh parar tudo

status:
	./servicos.sh status
