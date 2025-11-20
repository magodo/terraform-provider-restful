package acceptance

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/lfventura/terraform-provider-restful/internal/provider"
)

func ProviderFactory() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"restful": providerserver.NewProtocol6WithError(provider.New()),
		"echo":    echoprovider.NewProviderServer(),
	}
}
