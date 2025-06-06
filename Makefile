.PHONY: start build wire update new clean

SERVICE_NAME := essay.show
MODULE_NAME := essay-show

HANDLER_DIR := biz/adaptor/controller
MODEL_DIR := biz/application/dto
ROUTER_DIR := biz/adaptor/router

IDL_DIR ?= ../service-idl
MAIN_IDL_PATH := $(subst -,_,$(shell echo $(SERVICE_NAME) | awk -F '.' ' {for(i=1;i<=NF;i++) printf "%s/", $$i; printf "%s", $$NF }'))
FULL_MAIN_IDL_PATH := $(IDL_DIR)/$(MAIN_IDL_PATH).proto

IDL_DIR := "$(IDL_DIR)"
FULL_MAIN_IDL_PATH := "$(FULL_MAIN_IDL_PATH)"

IDL_OPTIONS := -I $(IDL_DIR) --idl $(FULL_MAIN_IDL_PATH)
OUTPUT_OPTIONS := --handler_dir $(HANDLER_DIR) --model_dir $(MODEL_DIR) --router_dir $(ROUTER_DIR)
EXTRA_OPTIONS := --pb_camel_json_tag=true --unset_omitempty=true

run:
	sh ./output/bootstrap.sh
build:
	sh ./build.sh
build_and_run:
	sh ./build.sh && sh ./output/bootstrap.sh
wire:
	wire ./provider
update:
	hz update $(IDL_OPTIONS) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
	@files=$$(find biz/application/dto -type f); \
	for file in $$files; do \
  	  sed -i  -e 's/func init\(\).*//' $$file; \
  	done
new:
	hz.exe new $(IDL_OPTIONS) $(OUTPUT_OPTIONS) --service $(SERVICE_NAME) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
clean:
	rm -r ./output
