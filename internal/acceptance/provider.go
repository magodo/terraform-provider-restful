package acceptance

import (
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/magodo/terraform-provider-restful/internal/provider"
)

func ProviderFactory() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"restful": func() (tfprotov6.ProviderServer, error) {
			return tfsdk.NewProtocol6Server(provider.New()), nil
		},
	}
}
