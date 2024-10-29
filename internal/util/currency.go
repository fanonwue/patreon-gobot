package util

import (
	"fmt"
	"math/big"
	"strings"
)

var currencySymbols = map[string]string{
	"EUR": "€",
	"USD": "$",
	"GBP": "£",
}

func CurrencySymbol(currency string) string {
	// Make sure the currency name is uppercase
	upper := strings.ToUpper(currency)
	symbol, found := currencySymbols[upper]
	if found {
		return symbol
	}
	return upper
}

func FormatMoney(money float64, currency string) string {
	symbol := CurrencySymbol(currency)
	return fmt.Sprintf("%.2f %s", money, symbol)
}

func FormatMoneyBig(money *big.Float, currency string) string {
	symbol := CurrencySymbol(currency)
	return money.Text('f', 2) + " " + symbol
}
