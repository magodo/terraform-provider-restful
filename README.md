# Terraform Provider Restful (Fork)

> **Note**: This is a fork of [magodo/terraform-provider-restful](https://github.com/magodo/terraform-provider-restful) with additional features.
> This version base was based initially with the magodo/terraform-provider-restful v0.24.0
> This forks adds the following functionality:
> * base_url and security can be defined inside the resource itself
> * adds sensitive_output option

This is a general Terraform provider aims to work for any platform as long as it exposes a RESTful API.

The document of this provider is available on [Terraform Provider Registry](https://registry.terraform.io/providers/lfventura/restful/latest/docs).

## Features

- Different authentication choices: HTTP auth (basic, token), API Key auth and OAuth2 (client credential, password credential, refresh token).
- Resource-level security configuration: Override provider-level authentication per resource for fine-grained control.
- Customized CRUD methods and paths
- Support precheck conditions
- Support polling asynchronous operations
- Partial `body` tracking: only the specified properties of the resource in the `body` attribute is tracked for diffs
- `restful_operation` resource that supports arbitrary Restful API call (e.g. `POST`) on create/update
- Ephemeral resource `restful_resource`
- [Write-only attributes](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments) supported

## Why

Given there already exists platform oriented, first-class providers, why do I create this? The reason is that most providers today are manually maintained, which means some latest features are likely not available in these first-class providers. For this case, `terraform-provider-restful` can be used as your escape hatch.

Another common use case is that the platform you are currently working on do not have a Terraform provider yet. In this case, you can use `terraform-provider-restful` to manage the resources for that platform.

## Requirement

`terraform-provider-restful` has following assumptions about the API:

- The API is expected to support the following HTTP methods:
    - `POST`/`PUT`: create the resource
    - `GET`: read the resource
    - `PUT`/`PATCH`/`POST`: update the resource
    - `DELETE`: remove the resource
- The API content type is `application/json`
- The resource should have a unique identifier (e.g. `/foos/foo1`).

Regarding the users, as `terraform-provider-restful` is essentially just a terraform-wrapped API client, practitioners have to know the details of the API for the target platform quite well.
