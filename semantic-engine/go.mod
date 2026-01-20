module github.com/futuristic-iac/semantic-engine

go 1.23.0
replace (
github.com/futuristic-iac/pkg/api => ../pkg/api
github.com/futuristic-iac/pkg/focus => ../pkg/focus
github.com/futuristic-iac/pkg/graph => ../pkg/graph
github.com/futuristic-iac/pkg/platform => ../pkg/platform
github.com/futuristic-iac/pkg/policy => ../pkg/policy
)
