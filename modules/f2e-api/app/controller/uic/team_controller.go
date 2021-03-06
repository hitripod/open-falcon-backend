package uic

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	h "github.com/Cepave/open-falcon-backend/modules/f2e-api/app/helper"
	"github.com/Cepave/open-falcon-backend/modules/f2e-api/app/model/uic"
	"github.com/Cepave/open-falcon-backend/modules/f2e-api/app/utils"
	"github.com/Cepave/open-falcon-backend/modules/f2e-api/config"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
)

type CTeam struct {
	Team        uic.Team
	TeamCreator string `json:"creator_name"`
	Useres      []uic.User
}

type APITeamInputs struct {
	Limit       int    `json:"limit" form:"limit"`
	Page        int    `json:"page" form:"page"`
	SkipMembers bool   `json:"skip_members" form:"skip_members"`
	Q           string `json:"q" form:"q"`
}

//support root as admin
func Teams(c *gin.Context) {
	inputs := APITeamInputs{
		Q:           ".+",
		Limit:       -1,
		Page:        -1,
		SkipMembers: false,
	}
	if err := c.Bind(&inputs); err != nil {
		h.JSONR(c, badstatus, err.Error())
		return
	}
	user, err := h.GetUser(c)
	if err != nil {
		h.JSONR(c, badstatus, err)
		return
	}
	var dt *gorm.DB
	var offset = 0
	teams := []uic.Team{}
	if user.IsAdmin() {
		dt = db.Uic
		if inputs.Limit != -1 && inputs.Page > 0 {
			if inputs.Page != 1 {
				offset = (inputs.Page - 1) * inputs.Limit
			}
			dt.Model(&teams).Where("name regexp ?", inputs.Q).Limit(inputs.Limit).Offset(offset).Scan(&teams)
		} else {
			dt = dt.Model(&teams).Where("name regexp ?", inputs.Q).Scan(&teams)
		}
	} else {
		dt = db.Uic.Model(&teams).Where("name regexp ? AND creator = ?", inputs.Q, user.ID).Scan(&teams)
		err = dt.Error
	}
	if err != nil {
		h.JSONR(c, badstatus, err)
		return
	}
	outputs := []CTeam{}
	for _, t := range teams {
		cteam := CTeam{Team: t}
		if !inputs.SkipMembers {
			user, err := t.Members()
			if err != nil {
				h.JSONR(c, badstatus, err)
				return
			}
			cteam.Useres = user
			creatorName, err := t.GetCreatorName()
			if err != nil {
				log.Debug(err.Error())
			}
			cteam.TeamCreator = creatorName
		}
		outputs = append(outputs, cteam)
	}
	h.JSONR(c, outputs)
	return
}

type APICreateTeamInput struct {
	Name    string  `json:"team_name" binding:"required"`
	Resume  string  `json:"resume"`
	UserIDs []int64 `json:"users"`
}

func CreateTeam(c *gin.Context) {
	var cteam APICreateTeamInput
	err := c.Bind(&cteam)
	//team_name is uniq column on db, so need check existing
	// team_name := c.DefaultQuery("team_name", "")
	if err != nil {
		h.JSONR(c, badstatus, err)
		return
	}
	user, err := h.GetUser(c)
	if err != nil {
		h.JSONR(c, badstatus, err)
		return
	} else if user.ID == 0 {
		h.JSONR(c, badstatus, "not found this user")
		return
	}
	team := uic.Team{
		Name:    cteam.Name,
		Resume:  cteam.Resume,
		Creator: user.ID,
	}
	dt := db.Uic.Save(&team)
	if dt.Error != nil {
		h.JSONR(c, badstatus, dt.Error)
		return
	}
	var dt2 *gorm.DB
	if len(cteam.UserIDs) > 0 {
		for i := 0; i < len(cteam.UserIDs); i++ {
			dt2 = db.Uic.Save(&uic.RelTeamUser{Tid: team.ID, Uid: cteam.UserIDs[i]})
			if dt2.Error != nil {
				err = dt2.Error
				break
			}
		}
		if err != nil {
			h.JSONR(c, badstatus, err)
			return
		}
	}
	h.JSONR(c, map[string]interface{}{
		"team": team,
		"msg":  fmt.Sprintf("team created! Afftect row: %d, Affect refer: %d", dt.RowsAffected, len(cteam.UserIDs)),
	})
	return
}

type APIUpdateTeamInput struct {
	ID      int    `json:"team_id" binding:"required"`
	Name    string `json:"team_name"`
	Resume  string `json:"resume"`
	UserIDs []int  `json:"users"`
}

func UpdateTeam(c *gin.Context) {
	cteam := APIUpdateTeamInput{Name: "-", Resume: "-"}
	err := c.Bind(&cteam)
	if err != nil {
		h.JSONR(c, badstatus, err)
		return
	}
	user, err := h.GetUser(c)
	dt := db.Uic.Table("team")
	if err != nil {
		dt.Rollback()
		h.JSONR(c, badstatus, err)
		return
	} else if user.IsAdmin() {
		dt = dt.Where("id = ?", cteam.ID)
	} else {
		dt = dt.Where("creator = ? AND id = ?", user.ID, cteam.ID)
	}
	var team uic.Team
	dt = dt.Find(&team)
	if dt.Error != nil {
		h.JSONR(c, badstatus, dt.Error)
		return
	}
	if team.ID != 0 {
		if cteam.Name != "-" {
			team.Name = cteam.Name
		}
		if cteam.Resume != "-" {
			team.Resume = cteam.Resume
		}
		db.Uic.Model(&team).Update(team)
		err = bindUsers(db, cteam.ID, cteam.UserIDs)
		if err != nil {
			h.JSONR(c, badstatus, "bind users got error: "+err.Error())
			return
		}
	}
	respOutput := APIGetTeamOutput{
		Team: team,
	}
	respOutput.Users, err = team.Members()
	if err != nil {
		h.JSONR(c, badstatus, err.Error())
		return
	}
	h.JSONR(c, respOutput)
	return
}

func bindUsers(db config.DBPool, tid int, users []int) (err error) {
	var dt *gorm.DB
	var uids string
	//delete unbind users
	var needDeleteMan []uic.RelTeamUser
	if len(users) != 0 {
		uids, err = utils.ArrIntToString(users)
		if err != nil {
			return
		}
		qPared := fmt.Sprintf("tid = %d AND NOT (uid IN (%v))", tid, uids)
		log.Debug(qPared)
		dt = db.Uic.Table("rel_team_user").Where(qPared).Find(&needDeleteMan)
		if dt.Error != nil {
			err = dt.Error
			return
		}
	}
	if len(needDeleteMan) != 0 {
		for _, man := range needDeleteMan {
			dt = db.Uic.Delete(&man)
			if dt.Error != nil {
				err = dt.Error
				return
			}
		}
	} else if len(users) == 0 && tid != 0 {
		rtmp := []uic.RelTeamUser{}
		dt = db.Uic.Model(&rtmp).Where("tid = ?", tid).Find(&rtmp)
		if dt.Error != nil {
			return dt.Error
		}
		db.Uic.Delete(&rtmp)
		return
	}
	//insert bind users
	for _, i := range users {
		ur := uic.RelTeamUser{Tid: int64(tid), Uid: int64(i)}
		db.Uic.Where(&ur).Find(&ur)
		if ur.ID == 0 {
			dt = db.Uic.Save(&ur)
		} else {
			//if record exsint, do next
			continue
		}
		if dt.Error != nil {
			err = dt.Error
			return
		}
	}
	return
}

type APIDeleteTeamInput struct {
	ID int64 `json:"team_id" binding:"required"`
}

func DeleteTeam(c *gin.Context) {
	var err error
	teamIdStr := c.Params.ByName("team_id")
	teamIdTmp, err := strconv.Atoi(teamIdStr)
	if err != nil {
		h.JSONR(c, badstatus, err.Error())
		return
	}
	teamId := int64(teamIdTmp)
	if teamId == 0 {
		h.JSONR(c, badstatus, "team_id is empty")
		return
	} else if err != nil {
		h.JSONR(c, badstatus, err)
		return
	}
	user, err := h.GetUser(c)
	if err != nil {
		h.JSONR(c, badstatus, err.Error())
		return
	}
	dt := db.Uic.Table("team")
	if user.IsAdmin() {
		dt = dt.Delete(&uic.Team{ID: teamId})
		err = dt.Error
	} else {
		team := uic.Team{
			ID:      teamId,
			Creator: user.ID,
		}
		dt = dt.Where(&team).Find(&team)
		if team.ID == 0 {
			err = errors.New("You don't have permission")
		} else if dt.Error != nil {
			err = dt.Error
		} else {
			db.Uic.Where("id = ?", teamId).Delete(&uic.Team{ID: teamId})
		}
	}
	var dt2 *gorm.DB
	if err != nil {
		h.JSONR(c, http.StatusExpectationFailed, err)
		return
	} else {
		dt2 = db.Uic.Where("tid = ?", teamId).Delete(uic.RelTeamUser{})
	}
	h.JSONR(c, fmt.Sprintf("team %v is deleted. Affect row: %d / refer delete: %d", teamId, dt.RowsAffected, dt2.RowsAffected))
	return
}

type APIGetTeamOutput struct {
	uic.Team
	Users []uic.User `json:"users"`
}

func GetTeam(c *gin.Context) {
	team_id_str := c.Params.ByName("team_id")
	team_id, err := strconv.Atoi(team_id_str)
	if team_id == 0 {
		h.JSONR(c, badstatus, "team_id is empty")
		return
	} else if err != nil {
		h.JSONR(c, badstatus, err)
		return
	}
	team := uic.Team{ID: int64(team_id)}
	dt := db.Uic.Find(&team)
	if dt.Error != nil {
		h.JSONR(c, badstatus, dt.Error)
		return
	}
	var uidarr []uic.RelTeamUser
	db.Uic.Table("rel_team_user").Select("uid").Where(&uic.RelTeamUser{Tid: int64(team_id)}).Find(&uidarr)
	if err != nil {
		log.Debug(err.Error())
	}
	var resp APIGetTeamOutput
	resp.Team = team
	resp.Users, err = resp.Members()
	if err != nil {
		h.JSONR(c, badstatus, err.Error())
		return
	}
	h.JSONR(c, resp)
	return
}

func GetTeamByName(c *gin.Context) {
	name := c.Params.ByName("team_name")
	if name == "" {
		h.JSONR(c, badstatus, "team name is missing")
		return
	}
	var team uic.Team

	dt := db.Uic.Table("team").Where(&uic.Team{Name: name}).Find(&team)
	if dt.Error != nil {
		h.JSONR(c, badstatus, dt.Error)
		return
	}

	var uidarr []uic.RelTeamUser
	dt = db.Uic.Table("rel_team_user").Select("uid").Where(&uic.RelTeamUser{Tid: team.ID}).Find(&uidarr)
	if dt.Error != nil {
		log.Debug(dt.Error)
	}
	var resp APIGetTeamOutput
	resp.Team = team
	resp.Users = []uic.User{}
	if len(uidarr) != 0 {
		uids := ""
		for indx, v := range uidarr {
			if indx == 0 {
				uids = fmt.Sprintf("%v", v.Uid)
			} else {
				uids = fmt.Sprintf("%v,%v", uids, v.Uid)
			}
		}
		log.Debugf("uids:%s", uids)
		var users []uic.User
		db.Uic.Table("user").Where(fmt.Sprintf("id IN (%s)", uids)).Find(&users)
		resp.Users = users
	}
	h.JSONR(c, resp)
	return
}
