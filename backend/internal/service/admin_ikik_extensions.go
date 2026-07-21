package service

func SetAdminUserPrivateGroupProvisioner(svc AdminService, provisioner UserPrivateGroupProvisioner) AdminService {
	if impl, ok := svc.(*adminServiceImpl); ok {
		impl.privateGroupProvisioner = provisioner
	}
	return svc
}
