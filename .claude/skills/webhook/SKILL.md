---
name: webhook
description: "Register webhook endpoints that forward HTTP requests to this chat. Use when user asks to create a webhook, set up an HTTP endpoint, receive API callbacks, integrate external services, listen for incoming requests, or manage existing webhooks (list, delete). Triggers on words like 'webhook', 'endpoint', 'callback', 'HTTP listener', 'API hook', 'incoming request'."
---

# Webhook Management

You can register webhook endpoints that forward incoming HTTP requests to this chat. When an external service calls the webhook URL, the request details are sent to Claude in this conversation for processing.

## Output Format

Wrap each webhook command in a fenced code block with language `nclaw:webhook`:

````
```nclaw:webhook
{"action":"create","description":"..."}
```
````

## Actions

### Create a Webhook

Fields:
- `action`: `"create"` (required)
- `description`: What this webhook is for (required). Used to provide context when processing incoming requests.

The system returns a URL like `https://{BASE_DOMAIN}/webhooks/{UUID}` that accepts any HTTP method (GET, POST, PUT, DELETE, etc.).

### Delete a Webhook

Fields:
- `action`: `"delete"` (required)
- `webhook_id`: UUID of the webhook to delete (required)

### List Webhooks

Fields:
- `action`: `"list"` (required)

Returns all webhooks registered in this chat with their IDs, descriptions, statuses, and URLs.

## How Incoming Requests Work

When an external service calls a webhook URL:
1. The server responds with 200 OK immediately.
2. In the background, a prompt is built containing the request method, headers, query parameters, body, and the webhook description.
3. Claude processes the request in this chat's session and sends the response back here.

## Rules

1. Always confirm to the user what webhook was created and provide the URL in your response text.
2. One command per code block. Use multiple code blocks for multiple actions.
3. Include the webhook code block naturally within your response - the system will strip it before showing the response to the user.
4. The description should be clear and specific, as it is included in the prompt when processing incoming requests.
5. When deleting, reference the webhook ID from the list of registered webhooks.

## Examples

User: "Create a webhook for GitHub push notifications"
Response: I've created a webhook endpoint for receiving GitHub push notifications. You can configure it in your GitHub repository's webhook settings.
```nclaw:webhook
{"action":"create","description":"GitHub push notifications - receives push events from a GitHub repository"}
```

User: "Set up an endpoint to receive form submissions"
Response: Done! Here's your webhook endpoint for receiving form submissions. Point your form's action URL to this endpoint.
```nclaw:webhook
{"action":"create","description":"Form submissions - receives HTML form POST data with user inputs"}
```

User: "List my webhooks"
Response: Let me check your registered webhooks.
```nclaw:webhook
{"action":"list"}
```

User: "Delete the GitHub webhook"
Response: I've deleted the GitHub push notifications webhook.
```nclaw:webhook
{"action":"delete","webhook_id":"a1b2c3d4-e5f6-7890-abcd-ef1234567890"}
```

User: "Create a webhook that monitors my deployment pipeline"
Response: I've set up a webhook to monitor your deployment pipeline. Configure your CI/CD system to send POST requests to this URL on deployment events.
```nclaw:webhook
{"action":"create","description":"Deployment pipeline monitor - receives CI/CD deployment status events (start, success, failure)"}
```
