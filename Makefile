.PHONY: help run run-schema

help:
	@echo 'run -- Run'

run:
	rm ./haven; go build haven; ./haven
