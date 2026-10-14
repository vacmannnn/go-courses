.PHONY: test check fmt

test:
	python3 -m unittest discover -s tests -v

check:
	go vet ./...
	python3 -m py_compile scripts/check.py scripts/ci.py scripts/advanced.py

fmt:
	gofmt -w task1/twosum_test.go.txt task2 task3 fixtures mockserver internal testdata
