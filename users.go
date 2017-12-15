package models

type User struct {
	DocumentBase `json:",inline" bson:",inline"`
	FirstName    string  `json:"name"  bson:"name"`
	LastName     string  `json:"status"  bson:"status"`
	Address      Address `json:"address" bson:"address"`
}

type Address struct {
	Email      string `json:"email"  bson:"email"`
	Phone      string `json:"phone"  bson:"phone"`
	Fax        string `json:"fax"  bson:"fax"`
	City       string `json:"city" bson:"city"`
	State      string `json:"state" bson:"state"`
	PostalCode string `json:"postalCode" bson:"postalCode"`
	Country    string `json:"country" bson:"country"`
}
