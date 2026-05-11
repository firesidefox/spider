---
description: Use when managing cron jobs on remote hosts — list, add, remove, or inspect scheduled tasks.
---

# Cron Job Management

Use this skill when the user asks about scheduled tasks, cron jobs, or periodic automation on remote hosts.

## When to invoke
- User asks to list, add, remove, or check cron jobs
- User wants to schedule a recurring task
- User asks why something runs periodically

## How to use
1. Use execute_cli to run crontab -l to list existing jobs
2. To add: (crontab -l 2>/dev/null; echo "*/5 * * * * /path/to/cmd") | crontab -
3. To remove: edit the crontab output and pipe back to crontab -
4. Check /etc/cron.d/ and /etc/cron.daily/ for system-level jobs
