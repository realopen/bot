package rmtstor

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sluggishhackers/go-realopen/models"
	"github.com/sluggishhackers/go-realopen/rmtstor/mysql"
	"github.com/sluggishhackers/go-realopen/utils/date"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

var REALOPEN_INDEX_DIR string = ".realopen-index"
var REALOPEN_DATA_DIR string = ".realopen-data"
var REALOPEN_INDEX_REPOSITORY string = "https://github.com/sluggishhackers/realopen-index.git"
var REALOPEN_DATA_REPOSITORY string

type IRemoteStorage interface {
	CreateBills(bills map[string]*models.Bill)
	FetchBillsNotOpened() []*mysql.Bill
	Initialize()
	SyncFilesRepository()
	UpdateBills([]*models.Bill)
	UploadFiles(bool)
}

type RemoteStorage struct {
	auth    *http.BasicAuth
	mysqlDb mysql.IMysql
}

func (rm *RemoteStorage) createBill(bill *models.Bill) {
	MemberID := os.Getenv("REALOPEN_MEMBER_ID")

	rm.mysqlDb.CreateBill(&mysql.Bill{
		BillID:                    bill.ID,
		Content:                   bill.OppCn,
		OpenType:                  bill.OppStleSeNm,
		OpenStatus:                bill.OppSeNm,
		ProcessorCode:             bill.ChrgDeptCd,
		ProcessorDepartmentName:   bill.PrcsDeptNm,
		ProcessorDrafterName:      bill.DrftrNmpn,
		ProcessorDrafterPosition:  bill.DrftrClsfNm,
		ProcessorName:             bill.PrcsNstNm,
		ProcessorRstrNumber:       bill.PrcsRstrNo,
		ProcessorStsCd:            bill.PrcsStsCd,
		ProcessorReviewerName:     bill.ChkrNmpn,
		ProcessorReviewerPosition: bill.ChkrClsfNm,
		RequestContent:            bill.RqestInfoDtls,
		RequestDate:               strings.ReplaceAll(bill.RqestPot, ".", "-"),
		BillTitle:                 bill.RqestSj,
		UserID:                    MemberID,
	})
}

func (rm *RemoteStorage) CreateBills(bills map[string]*models.Bill) {
	for _, b := range bills {
		rm.createBill(b)
	}
}

func (rm *RemoteStorage) FetchBillsNotOpened() []*mysql.Bill {
	return rm.mysqlDb.FetchBills("open_status = ?", "")
}

func (rm *RemoteStorage) Initialize() {
	// 1. Add a new user to database
	rm.initializeUser()

	REALOPEN_DATA_REPOSITORY = os.Getenv("REALOPEN_DATA_REPOSITORY_URL")

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	dataDir := fmt.Sprintf("%s/%s", wd, REALOPEN_DATA_DIR)

	cleanDataDirCmd := exec.Command("rm", "-rf", dataDir)
	cleanDataDirCmd.Run()

	_, err = git.PlainClone(dataDir, false, &git.CloneOptions{
		URL:      REALOPEN_DATA_REPOSITORY,
		Auth:     rm.auth,
		Progress: os.Stdout,
	})
	if err != nil {
		fmt.Errorf("😡 Error to clone REALOPEN_DATA_REPOSITORY", err)
		log.Fatal(err)
	}
}

func (rm *RemoteStorage) initializeUser() *mysql.User {
	username := os.Getenv("REALOPEN_MEMBER_NAME")
	memberID := os.Getenv("REALOPEN_MEMBER_ID")

	fmt.Println(username)
	fmt.Println(memberID)
	if username == "" {
		log.Fatal("NO USERNAME")
	}
	if memberID == "" {
		log.Fatal("NO MEMBER ID")
	}
	createdUser := rm.mysqlDb.FindOrCreateUser(&mysql.User{ID: memberID, Username: username})
	return createdUser
}

func (rm *RemoteStorage) SyncFilesRepository() {
	r, err := git.PlainOpen(REALOPEN_INDEX_DIR)
	if err != nil {
		log.Fatal(err)
	}

	w, err := r.Worktree()
	if err != nil {
		log.Fatal(err)
	}

	err = w.Pull(&git.PullOptions{
		Auth:     rm.auth,
		Progress: os.Stdout,
	})
	if err != nil && !strings.Contains(err.Error(), "already up-to-date") {
		fmt.Println("Failed to git pull - data repository")
		log.Fatal(err)
	}
}

func (rm *RemoteStorage) updateBill(bill *models.Bill) {
	fmt.Printf("Updated: %s\n", bill.ID)

	rm.mysqlDb.UpdateBill("bill_id = ?", bill.ID, &mysql.Bill{
		BillID:                    bill.ID,
		Content:                   "test",
		OpenType:                  bill.OppStleSeNm,
		OpenStatus:                bill.OppSeNm,
		ProcessorCode:             bill.ChrgDeptCd,
		ProcessorDepartmentName:   bill.PrcsDeptNm,
		ProcessorDrafterName:      bill.DrftrNmpn,
		ProcessorDrafterPosition:  bill.DrftrClsfNm,
		ProcessorName:             bill.PrcsNstNm,
		ProcessorRstrNumber:       bill.PrcsRstrNo,
		ProcessorStsCd:            bill.PrcsStsCd,
		ProcessorReviewerName:     bill.ChkrNmpn,
		ProcessorReviewerPosition: bill.ChkrClsfNm,
		RequestContent:            bill.RqestInfoDtls,
		RequestDate:               strings.ReplaceAll(bill.RqestPot, ".", "-"),
		BillTitle:                 bill.RqestSj,
	})

}

func (rm *RemoteStorage) UpdateBills(bills []*models.Bill) {
	for _, b := range bills {
		rm.updateBill(b)
	}
}

func (rm *RemoteStorage) UploadFiles(init bool) {
	var commitMsg string
	if init {
		commitMsg = "Welcome 🙌🏼"
	} else {
		commitMsg = fmt.Sprintf("UPDATED(%s)", date.Now().Format(date.DEFAULT_FORMAT))
	}

	r, err := git.PlainOpen(REALOPEN_DATA_DIR)
	if err != nil {
		log.Fatal(err)
	}

	w, err := r.Worktree()
	if err != nil {
		log.Fatal(err)
	}

	_, err = w.Add(".")
	if err != nil {
		fmt.Println("Failed to add")
		log.Fatal(err)
	}

	status, err := w.Status()
	if err != nil {
		fmt.Println("Failed to status")
		log.Fatal(err)
	}
	fmt.Println(status)

	commit, err := w.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name: rm.auth.Username,
			When: time.Now(),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	obj, err := r.CommitObject(commit)
	if err != nil {
		fmt.Println("Failed to commit")
		log.Fatal(err)
	}
	fmt.Println("Data Repository Commit: ")
	fmt.Println(obj)

	err = r.Push(&git.PushOptions{
		Auth:     rm.auth,
		Progress: os.Stdout,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Done: Push Data")
}

func New() IRemoteStorage {
	remoteStorage := &RemoteStorage{
		auth: &http.BasicAuth{
			Username: os.Getenv("REALOPEN_GIT_USERNAME"),
			Password: os.Getenv("REALOPEN_GIT_ACCESS_TOKEN"),
		},
	}

	remoteStorage.mysqlDb = mysql.New()

	return remoteStorage
}
