obj-m += hid_inzone_battery.o

KDIR ?= /lib/modules/$(shell uname -r)/build

all:
	$(MAKE) -C $(KDIR) M=$(CURDIR) CC=clang LLVM=1 modules

clean:
	$(MAKE) -C $(KDIR) M=$(CURDIR) clean
