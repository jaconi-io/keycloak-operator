package common

import (
	"context"

	kc "github.com/jaconi-io/keycloak-operator/api/v1alpha1"
	"github.com/jaconi-io/keycloak-operator/pkg/model"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UserState struct {
	User                 *kc.KeycloakAPIUser
	ClientRoles          map[string][]*kc.KeycloakUserRole
	RealmRoles           []*kc.KeycloakUserRole
	AvailableClientRoles map[string][]*kc.KeycloakUserRole
	AvailableRealmRoles  []*kc.KeycloakUserRole
	Clients              []*kc.KeycloakAPIClient
	Secret               *v1.Secret
	Keycloak             kc.Keycloak
	Context              context.Context
}

func NewUserState(keycloak kc.Keycloak) *UserState {
	return &UserState{
		ClientRoles:          map[string][]*kc.KeycloakUserRole{},
		AvailableClientRoles: map[string][]*kc.KeycloakUserRole{},
		Keycloak:             keycloak,
	}
}

func (i *UserState) Read(keycloakClient KeycloakInterface, userClient client.Client, user *kc.KeycloakUser, realm kc.KeycloakRealm) error {
	apiUser, err := i.readUser(keycloakClient, user, realm.Spec.Realm.Realm)
	if err != nil {
		// If there was an error reading the user then don't attempt
		// to read the roles. This user might not yet exist
		return nil
	}

	return i.ReadWithExistingAPIUser(keycloakClient, userClient, apiUser, realm)
}

func (i *UserState) ReadWithExistingAPIUser(keycloakClient KeycloakInterface, userClient client.Client, user *kc.KeycloakAPIUser, realm kc.KeycloakRealm) error {
	// Don't continue if the user could not be found
	if user == nil {
		return nil
	}

	i.User = user

	var err = i.readRealmRoles(keycloakClient, realm.Spec.Realm.Realm)
	if err != nil {
		return err
	}

	err = i.readClientRoles(keycloakClient, realm.Spec.Realm.Realm)
	if err != nil {
		return err
	}

	return i.readSecretState(userClient, &realm)
}

func (i *UserState) readUser(client KeycloakInterface, user *kc.KeycloakUser, realm string) (*kc.KeycloakAPIUser, error) {
	if user.Spec.User.ID != "" {
		keycloakUser, err := client.GetUser(user.Spec.User.ID, realm)
		if err != nil {
			return nil, err
		}
		return keycloakUser, nil
	}
	return nil, nil
}

func (i *UserState) readRealmRoles(client KeycloakInterface, realm string) error {
	// Get all the realm roles of this user
	roles, err := client.ListUserRealmRoles(realm, i.User.ID)
	if err != nil {
		return err
	}
	i.RealmRoles = roles

	// Get the roles that are still available to this user
	availableRoles, err := client.ListAvailableUserRealmRoles(realm, i.User.ID)
	if err != nil {
		return err
	}
	i.AvailableRealmRoles = availableRoles

	return nil
}

func (i *UserState) readClientRoles(client KeycloakInterface, realm string) error {
	clients, err := client.ListClients(realm)
	if err != nil {
		return err
	}
	i.Clients = clients

	for _, c := range clients {
		// Get all client roles of this user
		roles, err := client.ListUserClientRoles(realm, c.ID, i.User.ID)
		if err != nil {
			return err
		}
		i.ClientRoles[c.ClientID] = roles

		// Get the roles that are still available to this user
		availableRoles, err := client.ListAvailableUserClientRoles(realm, c.ID, i.User.ID)
		if err != nil {
			return err
		}
		i.AvailableClientRoles[c.ClientID] = availableRoles
	}
	return nil
}

func (i *UserState) readSecretState(userClient client.Client, realm *kc.KeycloakRealm) error {
	key := model.RealmCredentialSecretSelector(realm, i.User, &i.Keycloak)
	secret := &v1.Secret{}

	// Try to find the user credential secret
	err := userClient.Get(i.Context, key, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	i.Secret = secret
	return nil
}

// Check if a realm role is part of the available roles for this user
// Don't allow to assign unavailable roles
func (i *UserState) GetAvailableRealmRole(name string) *kc.KeycloakUserRole {
	for _, role := range i.AvailableRealmRoles {
		if role.Name == name {
			return role
		}
	}
	return nil
}

// Check if a client role is part of the available roles for this user
// Don't allow to assign unavailable roles
func (i *UserState) GetAvailableClientRole(name, clientID string) *kc.KeycloakUserRole {
	for _, role := range i.AvailableClientRoles[clientID] {
		if role.Name == name {
			return role
		}
	}
	return nil
}

// Keycloak clients have `ID` and `ClientID` properties and depending on the action we
// need one or the other. This function translates between the two
func (i *UserState) GetClientByID(clientID string) *kc.KeycloakAPIClient {
	for _, client := range i.Clients {
		if client.ClientID == clientID {
			return client
		}
	}
	return nil
}
