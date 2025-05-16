# Burn after reading

A secret sharing service written in Go using AWS Lambda and DynamoDB, and Terraform.

You provide a message and a password and get a link that you can share with someone.
In order to read the message, the recipient must provide the password. Once the message has been revealed it is deleted from the server. Unread messages expire in 24 hours.