package client

import (
	"context"
	"log"

	db_character "capturequest/internal/db/character"
	model "capturequest/internal/db/models"
	entity "capturequest/internal/zone/interface"
)

func (c *Client) SendStateUpdate() {
	if c.OnStateUpdate != nil {
		c.OnStateUpdate()
	}
}

var _ entity.Client = (*Client)(nil)

type Client struct {
	charData         *model.CharacterData
	options          *db_character.CharacterOptions
	ConnectionID     string
	OnSystemMessage  func(string)
	OnSpecialMessage func(string, string)
	OnStateUpdate    func()
}

func NewClient(charData *model.CharacterData, onSystemMessage func(string), onSpecialMessage func(string, string), onStateUpdate func()) (entity.Client, error) {
	opts, err := db_character.LoadOptions(context.Background(), int32(charData.ID))
	if err != nil {
		log.Printf("failed to load options for character %d, using defaults: %v", charData.ID, err)
		opts = db_character.DefaultOptions()
	}

	return &Client{
		charData:         charData,
		OnSystemMessage:  onSystemMessage,
		OnSpecialMessage: onSpecialMessage,
		OnStateUpdate:    onStateUpdate,
		options:          opts,
	}, nil
}

func (c *Client) CharData() *model.CharacterData {
	return c.charData
}

func (c *Client) Name() string {
	if c.charData == nil {
		return ""
	}
	return c.charData.Name
}

func (c *Client) Say(msg string) {
}

func (c *Client) ID() int {
	if c.charData == nil {
		return 0
	}
	return int(c.charData.ID)
}

func (c *Client) ShowNetworkStatsEnabled() bool {
	return c.options.ShowNetworkStats
}

func (c *Client) SetShowNetworkStatsEnabled(enabled bool) {
	c.options.ShowNetworkStats = enabled
}

func (c *Client) AllowTrainerRebattles() bool {
	return c.options.AllowTrainerRebattles
}

func (c *Client) SetAllowTrainerRebattlesEnabled(enabled bool) {
	c.options.AllowTrainerRebattles = enabled
}

func (c *Client) Options() interface{} {
	return c.options
}

func (c *Client) SaveOptions() error {
	return db_character.SaveOptions(context.Background(), int32(c.charData.ID), c.options)
}

func (c *Client) SendSystemMessage(text string) {
	if c.OnSystemMessage != nil {
		c.OnSystemMessage(text)
	}
}

func (c *Client) SendSpecialMessage(text string, msgType string) {
	if c.OnSpecialMessage != nil {
		c.OnSpecialMessage(text, msgType)
	}
}

func (c *Client) Shutdown() {
}
