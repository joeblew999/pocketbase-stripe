MUST use the task file !!!

MUST ensure the task steps for stripe are all that is needed, so a user only needs to work this way.

MUST ensure that all gitignores in each task file use vars where applicable to stay DRY !

MUST ensure that all taskfiles have a debug command that prints the vars.

MUST ensure that taskfiles are DRY. Each taskfile is responsible for only its stuff.

MUST ensure taskfile VARS and .env, process-compose.yml, DEVELOPMENT.md and Dockerfile stay in Sync. Task file vars are the source of truth.

MUST ensure that each task file include is idempotent. task file has amazing built in tools for this using command and status and other cool trick !!

MUST ensure taskfiles are cross-platform. Use {{exeExt}} for binaries (adds .exe on Windows). Prefix binary vars with namespace (e.g., PB_BINARY not BINARY).  
