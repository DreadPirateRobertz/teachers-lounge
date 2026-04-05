package model

// ShopItem describes a purchasable power-up in the gem shop catalog.
type ShopItem struct {
	Type        PowerUpType `json:"type"`
	Label       string      `json:"label"`
	Icon        string      `json:"icon"`
	Description string      `json:"description"`
	GemCost     int         `json:"gem_cost"`
}

// ShopCatalog is the full list of purchasable power-ups returned by GET /gaming/shop/catalog.
type ShopCatalog struct {
	Items []ShopItem `json:"items"`
}

// BuyPowerUpRequest is the request body for POST /gaming/shop/buy.
type BuyPowerUpRequest struct {
	UserID  string      `json:"user_id"`
	PowerUp PowerUpType `json:"power_up"`
}

// BuyPowerUpResponse is the response body for POST /gaming/shop/buy.
type BuyPowerUpResponse struct {
	// PowerUp is the type that was purchased.
	PowerUp PowerUpType `json:"power_up"`
	// NewCount is the buyer's updated inventory count for this power-up type.
	NewCount int `json:"new_count"`
	// GemsLeft is the buyer's gem balance after the purchase.
	GemsLeft int `json:"gems_left"`
}
