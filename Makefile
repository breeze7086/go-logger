.PHONY : all test

test_all = go test

ifndef $(func_name)
	test_cmd=$(test_all)
else
	test_cmd=$(test_all) -test.run $(func_name)
endif

all:
	@echo 'This is a Golang logging module'

test:
	@echo 'Module functions test...'
	@echo ''
	@if [ -e test.log ];then rm -f test.log;fi
	@$(test_cmd)
