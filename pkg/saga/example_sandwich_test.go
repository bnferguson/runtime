// Example: Sandwich Making
//
// This example demonstrates the fundamental mechanics of the saga framework
// using a whimsical sandwich-making domain. It's designed to be approachable
// and self-contained, showing how sagas work without requiring knowledge of
// our production systems.
//
// Key concepts demonstrated:
//
//   - Action chaining: Each action's output flows to dependent actions via saga keys
//   - Compensation: When AddProtein fails (out of turkey), completed actions are
//     undone in reverse order (UndoAddCondiment, UndoGetBread)
//   - Dependency injection: Collaborators (Kitchen, Pantry, Fridge) are injected
//     via Using() and retrieved with saga.Get[T](ctx)
//   - Optional inputs: The "toppings" input is marked optional, so the saga
//     succeeds even when it's omitted
//
// Run the examples:
//
//	go test -v ./pkg/saga/... -run Example
//
// See example_build_test.go for a more realistic example using the entity system.
package saga_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"miren.dev/runtime/pkg/saga"
)

// --- Collaborators ---

// Kitchen tracks what happened during sandwich making.
type Kitchen struct {
	log []string
}

func (k *Kitchen) record(action string) {
	k.log = append(k.log, action)
}

// Pantry holds bread and dry goods.
type Pantry struct {
	stock map[string]int
}

func NewPantry(stock map[string]int) *Pantry {
	return &Pantry{stock: stock}
}

func (p *Pantry) Take(item string) error {
	if p.stock[item] <= 0 {
		return fmt.Errorf("out of %s", item)
	}
	p.stock[item]--
	return nil
}

func (p *Pantry) Return(item string) {
	p.stock[item]++
}

// Fridge holds proteins, condiments, and cold items.
type Fridge struct {
	stock map[string]int
}

func NewFridge(stock map[string]int) *Fridge {
	return &Fridge{stock: stock}
}

func (f *Fridge) Take(item string) error {
	if f.stock[item] <= 0 {
		return fmt.Errorf("out of %s", item)
	}
	f.stock[item]--
	return nil
}

func (f *Fridge) Return(item string) {
	f.stock[item]++
}

// --- Actions ---

// GetBread retrieves bread from the pantry.

type GetBreadIn struct {
	BreadType string
}

type GetBreadOut struct {
	Bread string
}

func GetBread(ctx context.Context, in GetBreadIn) (GetBreadOut, error) {
	kitchen := saga.Get[*Kitchen](ctx)
	pantry := saga.Get[*Pantry](ctx)

	if err := pantry.Take(in.BreadType); err != nil {
		kitchen.record(fmt.Sprintf("Checked pantry - %v", err))
		return GetBreadOut{}, err
	}

	kitchen.record(fmt.Sprintf("Got %s from pantry", in.BreadType))
	return GetBreadOut{Bread: in.BreadType + " slice"}, nil
}

func UndoGetBread(ctx context.Context, in GetBreadIn, out GetBreadOut) error {
	kitchen := saga.Get[*Kitchen](ctx)
	pantry := saga.Get[*Pantry](ctx)

	pantry.Return(in.BreadType)
	kitchen.record(fmt.Sprintf("Returned %s to pantry", in.BreadType))
	return nil
}

// AddCondiment spreads a condiment on the bread.

type AddCondimentIn struct {
	Bread     string
	Condiment string
}

type AddCondimentOut struct {
	PreparedBread string
}

func AddCondiment(ctx context.Context, in AddCondimentIn) (AddCondimentOut, error) {
	kitchen := saga.Get[*Kitchen](ctx)
	fridge := saga.Get[*Fridge](ctx)

	if err := fridge.Take(in.Condiment); err != nil {
		kitchen.record(fmt.Sprintf("Checked fridge - %v", err))
		return AddCondimentOut{}, err
	}

	kitchen.record(fmt.Sprintf("Spread %s on %s", in.Condiment, in.Bread))
	return AddCondimentOut{
		PreparedBread: in.Bread + " with " + in.Condiment,
	}, nil
}

func UndoAddCondiment(ctx context.Context, in AddCondimentIn, out AddCondimentOut) error {
	kitchen := saga.Get[*Kitchen](ctx)
	fridge := saga.Get[*Fridge](ctx)

	fridge.Return(in.Condiment)
	kitchen.record(fmt.Sprintf("Scraped %s back into jar", in.Condiment))
	return nil
}

// AddProtein layers protein on the prepared bread.

type AddProteinIn struct {
	PreparedBread string
	Protein       string
}

type AddProteinOut struct {
	Stack string
}

func AddProtein(ctx context.Context, in AddProteinIn) (AddProteinOut, error) {
	kitchen := saga.Get[*Kitchen](ctx)
	fridge := saga.Get[*Fridge](ctx)

	if err := fridge.Take(in.Protein); err != nil {
		kitchen.record(fmt.Sprintf("Checked fridge - %v", err))
		return AddProteinOut{}, err
	}

	kitchen.record(fmt.Sprintf("Layered %s on %s", in.Protein, in.PreparedBread))
	return AddProteinOut{
		Stack: in.PreparedBread + " + " + in.Protein,
	}, nil
}

func UndoAddProtein(ctx context.Context, in AddProteinIn, out AddProteinOut) error {
	kitchen := saga.Get[*Kitchen](ctx)
	fridge := saga.Get[*Fridge](ctx)

	fridge.Return(in.Protein)
	kitchen.record(fmt.Sprintf("Put %s back in fridge", in.Protein))
	return nil
}

// AddToppings adds optional toppings to the sandwich.

type AddToppingsIn struct {
	Stack    string
	Toppings []string `saga:",optional"`
}

type AddToppingsOut struct {
	OpenSandwich string
}

func AddToppings(ctx context.Context, in AddToppingsIn) (AddToppingsOut, error) {
	kitchen := saga.Get[*Kitchen](ctx)

	result := in.Stack
	if len(in.Toppings) > 0 {
		toppingList := strings.Join(in.Toppings, ", ")
		kitchen.record(fmt.Sprintf("Added %s", toppingList))
		result += " + " + toppingList
	} else {
		kitchen.record("No toppings requested")
	}
	return AddToppingsOut{OpenSandwich: result}, nil
}

func UndoAddToppings(ctx context.Context, in AddToppingsIn, out AddToppingsOut) error {
	kitchen := saga.Get[*Kitchen](ctx)
	if len(in.Toppings) > 0 {
		kitchen.record(fmt.Sprintf("Removed %s", strings.Join(in.Toppings, ", ")))
	}
	return nil
}

// CloseSandwich adds the top slice to complete the sandwich.

type CloseSandwichIn struct {
	OpenSandwich string
}

type CloseSandwichOut struct {
	Sandwich string
}

func CloseSandwich(ctx context.Context, in CloseSandwichIn) (CloseSandwichOut, error) {
	kitchen := saga.Get[*Kitchen](ctx)
	kitchen.record("Closed sandwich with top slice")
	return CloseSandwichOut{
		Sandwich: "[" + in.OpenSandwich + "]",
	}, nil
}

func UndoCloseSandwich(ctx context.Context, in CloseSandwichIn, out CloseSandwichOut) error {
	kitchen := saga.Get[*Kitchen](ctx)
	kitchen.record("Opened sandwich back up")
	return nil
}

// Example_makeSandwich demonstrates a successful sandwich-making saga.
func Example_makeSandwich() {
	kitchen := &Kitchen{}
	pantry := NewPantry(map[string]int{
		"sourdough": 2,
		"wheat":     1,
		"rye":       1,
	})
	fridge := NewFridge(map[string]int{
		"mayo":     3,
		"mustard":  2,
		"ham":      4,
		"turkey":   2,
		"pastrami": 1,
	})

	registry := saga.NewRegistry()
	saga.Define("make-sandwich").
		Using(kitchen).
		Using(pantry).
		Using(fridge).
		Action(GetBread).Undo(UndoGetBread).
		Action(AddCondiment).Undo(UndoAddCondiment).
		Action(AddProtein).Undo(UndoAddProtein).
		Action(AddToppings).Undo(UndoAddToppings).
		Action(CloseSandwich).Undo(UndoCloseSandwich).
		RegisterTo(registry)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	ctx := context.Background()
	err := executor.Start("make-sandwich").
		Input("breadtype", "sourdough").
		Input("condiment", "mayo").
		Input("protein", "ham").
		Input("toppings", []string{"lettuce", "tomato"}).
		WithID("order-1").
		Execute(ctx)

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}

	// Get the final result
	exec, _ := storage.Get(ctx, "order-1")
	var result CloseSandwichOut
	json.Unmarshal(exec.ExecutedActions["close-sandwich"].Output, &result)

	fmt.Printf("Result: %s\n", result.Sandwich)

	fmt.Println("\nKitchen log:")
	for _, entry := range kitchen.log {
		fmt.Println("  -", entry)
	}

	// Output:
	// Result: [sourdough slice with mayo + ham + lettuce, tomato]
	//
	// Kitchen log:
	//   - Got sourdough from pantry
	//   - Spread mayo on sourdough slice
	//   - Layered ham on sourdough slice with mayo
	//   - Added lettuce, tomato
	//   - Closed sandwich with top slice
}

// Example_outOfStock demonstrates saga compensation when the fridge is empty.
func Example_outOfStock() {
	kitchen := &Kitchen{}
	pantry := NewPantry(map[string]int{
		"wheat": 1,
	})
	fridge := NewFridge(map[string]int{
		"mustard": 1,
		"turkey":  0, // Out of turkey!
	})

	registry := saga.NewRegistry()
	saga.Define("make-sandwich-fail").
		Using(kitchen).
		Using(pantry).
		Using(fridge).
		Action(GetBread).Undo(UndoGetBread).
		Action(AddCondiment).Undo(UndoAddCondiment).
		Action(AddProtein).Undo(UndoAddProtein).
		Action(AddToppings).Undo(UndoAddToppings).
		Action(CloseSandwich).Undo(UndoCloseSandwich).
		RegisterTo(registry)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	ctx := context.Background()
	err := executor.Start("make-sandwich-fail").
		Input("breadtype", "wheat").
		Input("condiment", "mustard").
		Input("protein", "turkey").
		Input("toppings", []string{"pickles"}).
		Execute(ctx)

	if err != nil {
		fmt.Println("Sandwich failed - Loss Prevented")
	}

	fmt.Println("\nKitchen log:")
	for _, entry := range kitchen.log {
		fmt.Println("  -", entry)
	}

	fmt.Printf("\nInventory restored: wheat=%d, mustard=%d\n",
		pantry.stock["wheat"], fridge.stock["mustard"])

	// Output:
	// Sandwich failed - Loss Prevented
	//
	// Kitchen log:
	//   - Got wheat from pantry
	//   - Spread mustard on wheat slice
	//   - Checked fridge - out of turkey
	//   - Scraped mustard back into jar
	//   - Returned wheat to pantry
	//
	// Inventory restored: wheat=1, mustard=1
}

// Example_simpleSandwich demonstrates optional toppings.
func Example_simpleSandwich() {
	kitchen := &Kitchen{}
	pantry := NewPantry(map[string]int{"rye": 1})
	fridge := NewFridge(map[string]int{"butter": 1, "pastrami": 1})

	registry := saga.NewRegistry()
	saga.Define("simple-sandwich").
		Using(kitchen).
		Using(pantry).
		Using(fridge).
		Action(GetBread).Undo(UndoGetBread).
		Action(AddCondiment).Undo(UndoAddCondiment).
		Action(AddProtein).Undo(UndoAddProtein).
		Action(AddToppings).Undo(UndoAddToppings).
		Action(CloseSandwich).Undo(UndoCloseSandwich).
		RegisterTo(registry)

	storage := saga.NewMemoryStorage()
	executor := saga.NewExecutor(storage, saga.WithRegistry(registry))

	ctx := context.Background()
	err := executor.Start("simple-sandwich").
		Input("breadtype", "rye").
		Input("condiment", "butter").
		Input("protein", "pastrami").
		// Note: no toppings - that's ok, it's optional!
		WithID("order-3").
		Execute(ctx)

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
		return
	}

	// Get the final result
	exec, _ := storage.Get(ctx, "order-3")
	var result CloseSandwichOut
	json.Unmarshal(exec.ExecutedActions["close-sandwich"].Output, &result)

	fmt.Printf("Result: %s\n", result.Sandwich)

	fmt.Println("\nKitchen log:")
	for _, entry := range kitchen.log {
		fmt.Println("  -", entry)
	}

	// Output:
	// Result: [rye slice with butter + pastrami]
	//
	// Kitchen log:
	//   - Got rye from pantry
	//   - Spread butter on rye slice
	//   - Layered pastrami on rye slice with butter
	//   - No toppings requested
	//   - Closed sandwich with top slice
}
