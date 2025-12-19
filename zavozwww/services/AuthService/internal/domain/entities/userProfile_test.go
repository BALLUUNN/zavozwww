package entities_test

import (
	"authServ/internal/domain/entities"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validProfileData returns valid data for creating a UserProfile.
// Returns: userID, username, firstName, lastName, age, info, city
func validProfileData() (uuid.UUID, string, string, string, int, string, string) {
	return uuid.New(), "johndoe", "John", "Doe", 30, "Some information about me.", "New York"
}

func TestNewUserProfile(t *testing.T) {
	t.Run("Успешное создание", func(t *testing.T) {
		userID, username, firstName, lastName, age, info, city := validProfileData()
		profile, err := entities.NewUserProfile(userID, username, firstName, lastName, age, info, city)

		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, userID, profile.UserId)
		assert.Equal(t, username, profile.Username)
		assert.Equal(t, firstName, profile.FirstName)
		assert.Equal(t, lastName, profile.LastName)
		assert.Equal(t, age, profile.Age)
		assert.Equal(t, info, profile.Info)
		assert.Equal(t, city, profile.City)
	})

	testCases := []struct {
		name        string
		userID      uuid.UUID
		username    string
		firstName   string
		lastName    string
		age         int
		info        string
		city        string
		expectedErr string
	}{
		{"Nil UserID", uuid.Nil, "validuser", "Valid", "Valid", 20, "Valid", "Valid", entities.ErrOfValidationUserID},
		{"Username слишком короткий", uuid.New(), "a", "Valid", "Valid", 20, "Valid", "Valid", "username must be between 2 and 50 characters"},
		{"Username слишком длинный", uuid.New(), strings.Repeat("a", 51), "Valid", "Valid", 20, "Valid", "Valid", "username must be between 2 and 50 characters"},
		{"Имя слишком короткое", uuid.New(), "validuser", "A", "Valid", 20, "Valid", "Valid", entities.ErrOfValidationFirstName},
		{"Имя слишком длинное", uuid.New(), "validuser", strings.Repeat("a", 51), "Valid", 20, "Valid", "Valid", entities.ErrOfValidationFirstName},
		{"Фамилия слишком короткая", uuid.New(), "validuser", "Valid", "D", 20, "Valid", "Valid", entities.ErrOfValidationLastName},
		{"Фамилия слишком длинная", uuid.New(), "validuser", "Valid", strings.Repeat("b", 51), 20, "Valid", "Valid", entities.ErrOfValidationLastName},
		{"Возраст слишком маленький", uuid.New(), "validuser", "Valid", "Valid", 5, "Valid", "Valid", entities.ErrOfValidationAge},
		{"Информация слишком длинная", uuid.New(), "validuser", "Valid", "Valid", 20, strings.Repeat("c", 5001), "Valid", entities.ErrOfValidationInfo},
		{"Город слишком длинный", uuid.New(), "validuser", "Valid", "Valid", 20, "Valid", strings.Repeat("d", 101), entities.ErrOfValidationCity},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entities.NewUserProfile(tc.userID, tc.username, tc.firstName, tc.lastName, tc.age, tc.info, tc.city)
			require.Error(t, err, "Конструктор должен возвращать ошибку для невалидных данных")
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestUserProfile_Validate(t *testing.T) {
	profile := &entities.UserProfile{
		UserId:    uuid.New(),
		Username:  "validuser",
		FirstName: "ValidFirstName",
		LastName:  "ValidLastName",
		Age:       25,
		Info:      "Valid info.",
		City:      "Valid City",
	}

	require.NoError(t, profile.Validate(), "Изначально созданный профиль должен быть валидным")

	originalFirstName := profile.FirstName
	profile.FirstName = "A"
	err := profile.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), entities.ErrOfValidationFirstName)
	profile.FirstName = originalFirstName

	originalUsername := profile.Username
	profile.Username = "a"
	err = profile.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username must be between 2 and 50 characters")
	profile.Username = originalUsername
}

func TestUserProfile_FullName(t *testing.T) {
	t.Run("Стандартное имя", func(t *testing.T) {
		profile := &entities.UserProfile{Username: "janesmith", FirstName: "Jane", LastName: "Smith"}
		assert.Equal(t, "janesmith: Jane Smith", profile.FullName())
	})

	t.Run("Имя с лишними пробелами", func(t *testing.T) {
		profile := &entities.UserProfile{Username: "peterjones", FirstName: "  Peter  ", LastName: "  Jones  "}
		assert.Equal(t, "peterjones: Peter Jones", profile.FullName())
	})
}

func TestUserProfile_String(t *testing.T) {
	userID, username, firstName, lastName, age, info, city := validProfileData()
	profile, err := entities.NewUserProfile(userID, username, firstName, lastName, age, info, city)
	require.NoError(t, err)

	expectedString := fmt.Sprintf("Профиль пользователя %s (ID: %s)", profile.FullName(), profile.UserId.String())
	assert.Equal(t, expectedString, profile.String())
}
