package util

import (
	"fmt"
	"math/big"
	"strings"
)

type Currency string

func (c Currency) Symbol() string {
	// Make sure the currency name is uppercase
	currencyString := c.String()
	switch currencyString {
	case "EUR":
		return "€"
	case "USD":
		return "$"
	case "GBP":
		return "£"
	default:
		return currencyString
	}
}

func (c Currency) String() string {
	return strings.ToUpper(string(c))
}

func CurrencySymbol(currency string) string {
	return Currency(currency).Symbol()
}

func FormatMoney(money float64, currency Currency) string {
	return fmt.Sprintf("%.2f %s", money, currency.Symbol())
}

func FormatMoneyBig(money *big.Float, currency Currency) string {
	return money.Text('f', 2) + " " + currency.Symbol()
}
