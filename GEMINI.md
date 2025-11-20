# Gemini Code Assistant Project Overview

## Project: terraform-provider-restful

This project is a Terraform provider for interacting with RESTful APIs. It's a generic provider that can be used to manage resources on any platform that exposes a RESTful API.

### Key Technologies

*   **Go:** The provider is written in Go.
*   **Terraform Plugin Framework:** It uses the `terraform-plugin-framework` for building the provider.
*   **Resty:** The `resty` library is used for making REST API calls.

### Architecture

The provider is structured as follows:

*   `main.go`: The entry point of the application. It initializes and serves the provider.
*   `internal/provider/provider.go`: The core of the provider. It defines the provider's schema, including all the configuration options for authentication, client behavior, and API methods. It also registers the resources and data sources.
*   `internal/provider/resource.go`: Implements the `restful_resource` resource, which is a flexible resource that allows users to manage any RESTful resource by defining its properties and the API paths for CRUD operations.
*   `internal/provider/operation_resource.go`: Implements the `restful_operation` resource, which allows for arbitrary RESTful API calls.
*   `internal/provider/data_source.go`: Implements the `restful_resource` data source, which allows for reading data from a RESTful API.
*   `internal/client/client.go`: A wrapper around the `resty` library that handles authentication and other client-side concerns.

### Building and Running

To build the provider, run the following command:

```bash
go build
```

To run the tests, run the following command:

```bash
go test ./...
```

### Development Conventions

*   The project follows the standard Go project layout.
*   It uses the `terraform-plugin-framework` for building the provider, which has its own set of conventions.
*   The code is well-documented with comments.
*   The project has a comprehensive test suite.
