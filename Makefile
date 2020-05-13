.PHONY: providers shell

common.mk:
	curl -s https://raw.githubusercontent.com/akaoj/common.mk/v1.0.3/common.mk -o $@

SERVICE_NAME = dico

include common.mk

providers:
	$(call container_make,providers,)

shell:
	$(call container_make,shell,)
