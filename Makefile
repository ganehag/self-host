BENCH_SCENARIO ?= bench/scenarios/local-smoke.yaml

all: aapije juvuln malgomaj

aapije:
	go build github.com/self-host/self-host/cmd/aapije

juvuln:
	go build github.com/self-host/self-host/cmd/juvuln

malgomaj:
	go build github.com/self-host/self-host/cmd/malgomaj

clean:
	rm aapije malgomaj juvuln

bench-local:
	./bench/run-local.sh $(BENCH_SCENARIO)

bench-smoke:
	./bench/run-local.sh bench/scenarios/local-smoke.yaml

bench-read-heavy:
	./bench/run-local.sh bench/scenarios/read-heavy.yaml

bench-mixed:
	./bench/run-local.sh bench/scenarios/mixed.yaml

bench-write-heavy:
	./bench/run-local.sh bench/scenarios/write-heavy.yaml

bench-down:
	./bench/down-local.sh

seaweedfs-test:
	./test/seaweedfs/run-local.sh

seaweedfs-down:
	./test/seaweedfs/down-local.sh
