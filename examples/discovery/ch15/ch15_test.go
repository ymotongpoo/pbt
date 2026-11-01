// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch15

import (
	"errors"
	"fmt"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
	"go.ymotongpoo.com/pbt/fsm"
)

// Account は new、verified、locked の3つの状態を移る。
type Account struct{ state string }

var errState = errors.New("not allowed in this state")

func (a *Account) Verify() error { return a.move("new", "verified") }
func (a *Account) Lock() error   { return a.move("verified", "locked") }
func (a *Account) Unlock() error { return a.move("locked", "verified") }

func (a *Account) move(from, to string) error {
	if a.state != from {
		return errState
	}
	a.state = to
	return nil
}

type account interface {
	Verify() error
	Lock() error
	Unlock() error
}

// command は操作と期待するエラーを1つのコマンドに束ねる。
func command(name string, call func() error, want error) pbt.CommandGen {
	return pbt.Just(pbt.Command{
		Name: name,
		Run:  func(pbt.Env) any { return call() },
		Post: func(_ pbt.Env, got any) error {
			err, _ := got.(error)
			if !errors.Is(err, want) {
				return fmt.Errorf("%s returned %v, want %v", name, err, want)
			}
			return nil
		},
	})
}

// sixCells は、表の6マスだけを遷移にした機械である。
func sixCells(a account) func(string) []fsm.Transition {
	return func(state string) []fsm.Transition {
		switch state {
		case "new":
			return []fsm.Transition{
				{Weight: 3, To: "verified", Cmd: command("verify", a.Verify, nil)},
				{Weight: 1, To: "new", Cmd: command("lock", a.Lock, errState)},
			}
		case "verified":
			return []fsm.Transition{
				{Weight: 2, To: "locked", Cmd: command("lock", a.Lock, nil)},
				{Weight: 1, To: "verified", Cmd: command("verify", a.Verify, errState)},
			}
		default: // locked
			return []fsm.Transition{
				{Weight: 2, To: "verified", Cmd: command("unlock", a.Unlock, nil)},
				{Weight: 1, To: "locked", Cmd: command("lock", a.Lock, errState)},
			}
		}
	}
}

// nineCells は、状態と操作のすべての組を遷移にした機械である。
func nineCells(a account) func(string) []fsm.Transition {
	return func(state string) []fsm.Transition {
		switch state {
		case "new":
			return []fsm.Transition{
				{Weight: 3, To: "verified", Cmd: command("verify", a.Verify, nil)},
				{Weight: 1, To: "new", Cmd: command("lock", a.Lock, errState)},
				{Weight: 1, To: "new", Cmd: command("unlock", a.Unlock, errState)},
			}
		case "verified":
			return []fsm.Transition{
				{Weight: 2, To: "locked", Cmd: command("lock", a.Lock, nil)},
				{Weight: 1, To: "verified", Cmd: command("verify", a.Verify, errState)},
				{Weight: 1, To: "verified", Cmd: command("unlock", a.Unlock, errState)},
			}
		default: // locked
			return []fsm.Transition{
				{Weight: 2, To: "verified", Cmd: command("unlock", a.Unlock, nil)},
				{Weight: 1, To: "locked", Cmd: command("verify", a.Verify, errState)},
				{Weight: 1, To: "locked", Cmd: command("lock", a.Lock, errState)},
			}
		}
	}
}

// nineCellsStayNew は、new に留まる遷移の重みを上げた機械である。
func nineCellsStayNew(a account) func(string) []fsm.Transition {
	table := nineCells(a)
	return func(state string) []fsm.Transition {
		if state != "new" {
			return table(state)
		}
		return []fsm.Transition{
			{Weight: 1, To: "verified", Cmd: command("verify", a.Verify, nil)},
			{Weight: 2, To: "new", Cmd: command("lock", a.Lock, errState)},
			{Weight: 2, To: "new", Cmd: command("unlock", a.Unlock, errState)},
		}
	}
}

func accountProperty(
	newAccount func() account,
	machine func(account) func(string) []fsm.Transition,
) pbt.Property {
	return func(t *pbt.T) {
		a := newAccount()
		prog := fsm.Commands("new", nil, machine(a)).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func correct() account { return &Account{state: "new"} }

func TestAccountSixCells(t *testing.T)  { pbt.Check(t, accountProperty(correct, sixCells)) }
func TestAccountNineCells(t *testing.T) { pbt.Check(t, accountProperty(correct, nineCells)) }

func TestAccountStats(t *testing.T) {
	pbt.Check(t, accountProperty(correct, nineCells), pbt.Seed(1), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
}
func TestAccountStatsStayNew(t *testing.T) {
	pbt.Check(t, accountProperty(correct, nineCellsStayNew), pbt.Seed(1), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
}

// lenientAccount は、locked のままでも Verify が成功する変異である。
type lenientAccount struct{ Account }

func (a *lenientAccount) Verify() error {
	if a.state == "locked" {
		a.state = "verified"
		return nil
	}
	return a.Account.Verify()
}

func lenient() account { return &lenientAccount{Account{state: "new"}} }

func TestLenientSixCellsSurvives(t *testing.T) {
	pbt.Check(t, accountProperty(lenient, sixCells), pbt.Seed(1), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
}
func TestLenientNineCellsMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, accountProperty(lenient, nineCells), pbt.Seed(1), pbt.NoSave(), pbt.NoReplay())
}

// silentAccount は、拒否した Lock のあとで状態だけ進めてしまう変異である。
type silentAccount struct{ Account }

func (a *silentAccount) Lock() error {
	if a.state == "new" {
		a.state = "verified"
		return errState
	}
	return a.Account.Lock()
}

func TestSilentMoveMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, accountProperty(func() account { return &silentAccount{Account{state: "new"}} }, nineCells), pbt.Seed(1), pbt.NoSave(), pbt.NoReplay())
}

// accountCommandProperty は、同じアカウントをコマンド方式で書いたものである。
func accountCommandProperty(newAccount func() account) pbt.Property {
	return func(t *pbt.T) {
		a := newAccount()
		state := "new" // モデル：今の状態の名前

		op := func(name string, call func() error, from, to string) pbt.CommandGen {
			return pbt.Just(pbt.Command{
				Name: name,
				Run:  func(pbt.Env) any { return call() },
				Next: func(pbt.Var) {
					if state == from {
						state = to
					}
				},
				Post: func(_ pbt.Env, got any) error {
					err, _ := got.(error)
					want := errState
					if state == from {
						want = nil
					}
					if !errors.Is(err, want) {
						return fmt.Errorf("%s in %s returned %v, want %v", name, state, err, want)
					}
					return nil
				},
			})
		}
		verify := op("verify", a.Verify, "new", "verified")
		lock := op("lock", a.Lock, "verified", "locked")
		unlock := op("unlock", a.Unlock, "locked", "verified")

		commands := func() pbt.CommandGen { return pbt.OneOf(verify, lock, unlock) }
		prog := pbt.Commands(func() { state = "new" }, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// accountCommandWeighted は、状態ごとに重みを付けたコマンド方式である。
func accountCommandWeighted(newAccount func() account) pbt.Property {
	return func(t *pbt.T) {
		a := newAccount()
		state := "new" // モデル：今の状態の名前

		op := func(name string, call func() error, from, to string) pbt.CommandGen {
			return pbt.Just(pbt.Command{
				Name: name,
				Run:  func(pbt.Env) any { return call() },
				Next: func(pbt.Var) {
					if state == from {
						state = to
					}
				},
				Post: func(_ pbt.Env, got any) error {
					err, _ := got.(error)
					want := errState
					if state == from {
						want = nil
					}
					if !errors.Is(err, want) {
						return fmt.Errorf("%s in %s returned %v, want %v", name, state, err, want)
					}
					return nil
				},
			})
		}
		verify := op("verify", a.Verify, "new", "verified")
		lock := op("lock", a.Lock, "verified", "locked")
		unlock := op("unlock", a.Unlock, "locked", "verified")

		commands := func() pbt.CommandGen {
			switch state {
			case "new":
				return pbt.Frequency(pbt.W(3, verify), pbt.W(1, lock), pbt.W(1, unlock))
			case "verified":
				return pbt.Frequency(pbt.W(2, lock), pbt.W(1, verify), pbt.W(1, unlock))
			default: // locked
				return pbt.Frequency(pbt.W(2, unlock), pbt.W(1, verify), pbt.W(1, lock))
			}
		}
		prog := pbt.Commands(func() { state = "new" }, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestAccountCommandWeighted(t *testing.T) {
	pbt.Check(t, accountCommandWeighted(correct), pbt.Seed(1), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
}

func TestAccountCommandStyle(t *testing.T) {
	pbt.Check(t, accountCommandProperty(correct), pbt.Seed(1), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
}
func TestLenientCommandStyleMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, accountCommandProperty(lenient), pbt.Seed(1), pbt.NoSave(), pbt.NoReplay())
}

// ---- 状態データを持つ機械

const maxFailures = 3

var (
	errPassword = errors.New("wrong password")
	errLocked   = errors.New("account is locked")
)

// LoginAccount は、パスワードを maxFailures 回続けて間違えるとロックされる。
type LoginAccount struct {
	password string
	failures int
	locked   bool
}

func (a *LoginAccount) Login(password string) error {
	if a.locked {
		return errLocked
	}
	if password != a.password {
		a.failures++
		if a.failures >= maxFailures {
			a.locked = true
		}
		return errPassword
	}
	a.failures = 0
	return nil
}

// Reset はロックを解除する。ロックされていなければエラーを返す。
func (a *LoginAccount) Reset() error {
	if !a.locked {
		return errState
	}
	a.locked = false
	a.failures = 0
	return nil
}

type loginAccount interface {
	Login(password string) error
	Reset() error
}

const secret = "secret"

// loginMachine は、機械の初期化関数と遷移の関数を返す。
func loginMachine(a loginAccount) (func(), func(string) []fsm.Transition) {
	failures := 0 // 状態データ：続けて間違えた回数

	// step は、状態データを更新する遷移のコマンドを作る。
	step := func(name string, call func() error, want error, pre func() bool, next func()) pbt.CommandGen {
		return pbt.Map(command(name, call, want), func(c pbt.Command) pbt.Command {
			c.Pre = pre
			c.Next = func(pbt.Var) { next() }
			return c
		})
	}
	login := func(password string) func() error {
		return func() error { return a.Login(password) }
	}
	belowLimit := func() bool { return failures < maxFailures-1 }
	atLimit := func() bool { return failures == maxFailures-1 }

	good := step("login_ok", login(secret), nil, nil, func() { failures = 0 })
	bad := step("login_bad", login("wrong"), errPassword, belowLimit, func() { failures++ })
	last := step("login_last", login("wrong"), errPassword, atLimit, func() { failures++ })
	reset := step("reset", a.Reset, nil, nil, func() { failures = 0 })

	transitions := func(state string) []fsm.Transition {
		if state == "locked" {
			return []fsm.Transition{
				{Weight: 2, To: "open", Cmd: reset},
				{Weight: 1, To: "locked", Cmd: command("login_ok", login(secret), errLocked)},
				{Weight: 1, To: "locked", Cmd: command("login_bad", login("wrong"), errLocked)},
			}
		}
		wrong := fsm.Transition{Weight: 3, To: "open", Cmd: bad}
		if atLimit() {
			wrong = fsm.Transition{Weight: 3, To: "locked", Cmd: last}
		}
		return []fsm.Transition{
			{Weight: 1, To: "open", Cmd: good},
			wrong,
			{Weight: 1, To: "open", Cmd: command("reset", a.Reset, errState)},
		}
	}
	return func() { failures = 0 }, transitions
}

func loginProperty(newAccount func() loginAccount) pbt.Property {
	return func(t *pbt.T) {
		init, transitions := loginMachine(newAccount())
		prog := fsm.Commands("open", init, transitions).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func correctLogin() loginAccount { return &LoginAccount{password: secret} }

func TestLoginMachine(t *testing.T) {
	pbt.Check(t, loginProperty(correctLogin), pbt.Seed(1), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
}

// stickyFailures は、ログインに成功しても失敗回数を戻さない変異である。
type stickyFailures struct{ LoginAccount }

func (a *stickyFailures) Login(password string) error {
	saved := a.failures
	err := a.LoginAccount.Login(password)
	if err == nil {
		a.failures = saved
	}
	return err
}

// offByOne は、4回目の失敗でロックする変異である。
type offByOne struct{ LoginAccount }

func (a *offByOne) Login(password string) error {
	if !a.locked && password != a.password && a.failures == maxFailures-1 {
		a.failures++
		return errPassword
	}
	if !a.locked && password != a.password && a.failures == maxFailures {
		a.locked = true
		return errPassword
	}
	return a.LoginAccount.Login(password)
}

// keepOnReset は、解除しても失敗回数を戻さない変異である。
type keepOnReset struct{ LoginAccount }

func (a *keepOnReset) Reset() error {
	saved := a.failures
	err := a.LoginAccount.Reset()
	a.failures = saved
	return err
}

func TestStickyFailuresMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, loginProperty(func() loginAccount { return &stickyFailures{LoginAccount{password: secret}} }), pbt.Seed(1), pbt.NoSave(), pbt.NoReplay())
}
func TestOffByOneMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, loginProperty(func() loginAccount { return &offByOne{LoginAccount{password: secret}} }), pbt.Seed(1), pbt.NoSave(), pbt.NoReplay())
}
func TestKeepOnResetMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, loginProperty(func() loginAccount { return &keepOnReset{LoginAccount{password: secret}} }), pbt.Seed(1), pbt.NoSave(), pbt.NoReplay())
}

// init を渡さないと、生成のときに進んだ回数が実行のときに残る。
func TestLoginNoInitMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		_, transitions := loginMachine(correctLogin())
		prog := fsm.Commands("open", nil, transitions).Draw(t, "cmds")
		prog.RunSequential(t)
	}, pbt.NoSave(), pbt.NoReplay())
}
