package auth

/*
func TestCreateAzureAuth(t *testing.C) {
	config := AzureAuthConfig{
		Role: "test-role",
		Resource: "test-resource",
		Path: "test-path",
	}

	expectedAuthMethod, err := azure.NewAzureAuth(
		config.Role,
		azure.WithResource(config.Resource),
		azure.WithMountPath(config.Path),
	)
	assert.NoError(t, err, "NewAzureAuth failed unexpectedly")

	VaultAuth, err := createAzureAuth(config)
	assert.NoError(t, err, "createAzureAuth failed unexpectedly")

	assert.Equal(t, expectedAuthMethod, VaultAuth.delegate)
}
*/
