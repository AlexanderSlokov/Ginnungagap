# Forward log Falco to log file to tuning rules
falco_up:
	docker compose up falco -d

forward_falco_log:falco_up
	docker compose logs -f falco > logs/falco_debug.txt

sandbox_up:
	docker compose up sandbox -d

forward_sandbox_log:falco_up
	docker compose logs -f sandbox > logs/sandbox_debug.txt