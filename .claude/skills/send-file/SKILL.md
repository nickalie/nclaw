---
name: send-file
description: "Send files back to the user via Telegram. Use when Claude creates, generates, or modifies a file that the user needs to receive (e.g., generated reports, exported data, created images, code files, archives). Triggers on requests like 'send me the file', 'export as PDF', 'generate a report', 'create a file and send it', or whenever Claude produces a file artifact the user should receive."
---

# Send File to User

You can send files to the user by embedding send-file commands in your response. The system parses these commands and delivers the files via Telegram.

## Output Format

Wrap each send-file command in a fenced code block with language `nclaw:sendfile`:

````
```nclaw:sendfile
{"path":"relative/or/absolute/path/to/file"}
```
````

## Fields

- `path`: Path to the file to send (required). Can be:
  - Relative to the current working directory (e.g., `output.csv`, `reports/summary.pdf`)
  - Absolute path (e.g., `/tmp/generated.png`)
- `caption`: Short description shown with the file in Telegram (optional, max 1024 chars)

## Rules

1. The file must exist on disk before you emit the send-file block. Create or generate it first.
2. One file per code block. Use multiple code blocks for multiple files.
3. Include the send-file block naturally within your response — the system will strip it before showing the response to the user.
4. Always tell the user what file you're sending in your response text.

## Examples

User: "Generate a CSV of prime numbers under 100 and send it to me"
Response: Here's the CSV with all prime numbers under 100!
```nclaw:sendfile
{"path":"primes.csv","caption":"Prime numbers under 100"}
```

User: "Create a Python script that sorts a list"
Response: I've created the sorting script for you. Sending it now!
```nclaw:sendfile
{"path":"sort_list.py","caption":"List sorting script"}
```

User: "Export the analysis results"
Response: Done! Here are your analysis results.
```nclaw:sendfile
{"path":"results/analysis.xlsx","caption":"Analysis results"}
```
```nclaw:sendfile
{"path":"results/charts.png","caption":"Analysis charts"}
```
