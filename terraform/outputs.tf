output "lambda_function_name" {
  value = aws_lambda_function.burn_after_reading.function_name
}

output "dynamodb_table_name" {
  value = aws_dynamodb_table.snippets.name
}

output "lambda_function_url" {
  value = aws_lambda_function_url.burn_after_reading_url.function_url
}
