package handler

import (
	"project-manager/backend/internal/model"

	"gorm.io/gorm"
)

func findUsersByIDs(tx *gorm.DB, ids []uint) ([]model.User, error) {
	var users []model.User
	if len(ids) == 0 {
		return users, nil
	}
	if err := tx.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func findDepartmentsByIDs(tx *gorm.DB, ids []uint) ([]model.Department, error) {
	var departments []model.Department
	if len(ids) == 0 {
		return departments, nil
	}
	if err := tx.Where("id IN ?", ids).Find(&departments).Error; err != nil {
		return nil, err
	}
	return departments, nil
}

func findRolesByIDs(tx *gorm.DB, ids []uint) ([]model.Role, error) {
	var roles []model.Role
	if len(ids) == 0 {
		return roles, nil
	}
	if err := tx.Where("id IN ?", ids).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func findPermissionsByIDs(tx *gorm.DB, ids []uint) ([]model.Permission, error) {
	var permissions []model.Permission
	if len(ids) == 0 {
		return permissions, nil
	}
	if err := tx.Where("id IN ?", ids).Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

func replaceAssociation(tx *gorm.DB, owner interface{}, association string, values interface{}) error {
	return tx.Model(owner).Association(association).Replace(values)
}

func clearAssociation(tx *gorm.DB, owner interface{}, association string) error {
	return tx.Model(owner).Association(association).Clear()
}
