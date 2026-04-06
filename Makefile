.PHONY: notify-test

PROVIDER ?= auto

notify-test:
	bash scripts/test-task-notify.sh $(PROVIDER)
