#!/bin/sh
update() {
  timeout 60 claude update || echo "claude update failed, keeping current version" >&2
}

update
(while sleep 86400; do update; done) &
exec nclaw "$@"
