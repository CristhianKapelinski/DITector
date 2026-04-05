.PHONY: build clean init start stop update logs status

BINARY = ditector

# ── Build local ───────────────────────────────────────────────────────────────
build:
	@GOARCH=amd64 go build -o $(BINARY)

clean:
	@rm -f $(BINARY)

rebuild: clean build

# ── Configuração: lê .env se existir ─────────────────────────────────────────
-include .env
export

ifeq ($(ROLE),secondary)
  _MONGO    := mongodb://$(DB_HOST):27017
  _NEO4J    := neo4j://$(DB_HOST):7687
  _PROFILES :=
else
  _MONGO    := mongodb://localhost:27017
  _NEO4J    := neo4j://localhost:7687
  _PROFILES := --profile db
endif

# ── Operação ──────────────────────────────────────────────────────────────────
init:
	@[ -f .env ] && echo ".env já existe." || (cp .env.example .env && echo "Criado .env — edite antes de continuar.")

start:
	@[ -f .env ] || (echo "Execute 'make init' primeiro." && exit 1)
	MONGO_URI=$(_MONGO) NEO4J_URI=$(_NEO4J) \
	  docker-compose $(_PROFILES) up -d --force-recreate --remove-orphans crawler \
	  $(if $(_PROFILES),mongodb neo4j,)

stop:
	docker-compose $(_PROFILES) stop

clean-containers:
	@echo "Limpando containers fantasmas do projeto..."
	@docker ps -a | grep ditector | awk '{print $$1}' | xargs -r docker rm -f

update:
	git fetch origin master && git reset --hard origin/master
	@$(MAKE) start

start-build:
	@[ -f .env ] || (echo "Execute 'make init' primeiro." && exit 1)
	MONGO_URI=$(_MONGO) NEO4J_URI=$(_NEO4J) \
	  docker-compose -f docker-compose.node3.yml up -d --force-recreate --remove-orphans builder

logs:
	@docker logs -f ditector_crawler 2>&1 | grep -E "Flushed|WARN|ERROR|401|429"

logs-build:
	@docker logs -f ditector_builder 2>&1

status:
	@mongosh $(_MONGO)/dockerhub_data --quiet --eval \
	  'print("repos:", db.repositories_data.countDocuments(), \
	         " | keywords:", db.crawler_keywords.countDocuments(), \
	         " | graph_built:", db.repositories_data.countDocuments({graph_built_at:{$$exists:true}}))'
