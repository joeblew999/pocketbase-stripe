MUST use the task file !!!

MUST ensure the task steps for stripe are all that is needed, so a user only needs to work this way.

MUST ensure that all gitignores in each task file  use vars where applicable to stay DRY !

MUST ensure that al tasks file has a debug command that prints the vars. Really helpful for debugging, and need less echo crap in a taskfile.

MUST ensure that taskfiles are DRY. When refactoring make sure that each taskfile is responsibel for only its stuff, and dont leave old task commands in root task file when you add them to task file includes.


MUST ensure taskfile VARS and .env, process-compose.yml, DEVELOPMENT.md and Dockerfile thay in Sync. Task file vars are the source of truth, since we gen from there .

