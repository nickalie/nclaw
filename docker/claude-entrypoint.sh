#!/bin/sh
timeout 60 claude update || echo "claude update failed, using bundled version" >&2
exec nclaw "$@"
